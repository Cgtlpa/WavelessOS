package main

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

func recipeVars(dir string) map[string]string {
	vars := make(map[string]string)
	f, err := os.Open(dir + "/recipe")
	if err != nil {
		return vars
	}
	defer f.Close()

	scan := bufio.NewScanner(f)
	for scan.Scan() {
		line := strings.TrimSpace(scan.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		vars[strings.TrimSpace(parts[0])] = strings.TrimSpace(parts[1])
	}
	return vars
}

func sysroot() string {
	if r := os.Getenv("WAV_SYSROOT"); r != "" {
		return r
	}
	return "/usr/local/waveless"
}

func cachedir() string {
	if c := os.Getenv("WAV_CACHE"); c != "" {
		return c
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "/tmp/wav-cache"
	}
	return home + "/.wav/cache"
}

func tmpdir() string {
	if t := os.Getenv("WAV_TMP"); t != "" {
		return t
	}
	return "/tmp/wav-build"
}

func jobs() string {
	if j := os.Getenv("WAV_JOBS"); j != "" {
		return j
	}
	return fmt.Sprintf("%d", runtime.NumCPU())
}

func acquire(pkg string) {
	if acquirePrebuilt(pkg) {
		return
	}
	acquireSource(pkg)
}

func acquireSource(pkg string) {
	dir, ok := pkgDir(pkg)
	if !ok {
		die("package %s not found", pkg)
	}

	vars := recipeVars(dir)
	name := vars["name"]
	if name == "" {
		name = pkg
	}
	version := vars["version"]
	source := vars["source"]

	banner("acquiring " + name)

	if source != "" {
		err := downloadSource(name, version, source)
		if err != nil {
			fatalln(err)
		}
	}

	extractDir := tmpdir() + "/" + name
	if version != "" {
		extractDir += "-" + version
	}
	os.MkdirAll(extractDir, 0755)

	wavSrc := extractDir + "/src"
	if source != "" {
		err := extractSource(source, wavSrc)
		if err != nil {
			fatalln(err)
		}
	}

	actualSrc := findActualSrcDir(wavSrc)
	if actualSrc != "" {
		wavSrc = actualSrc
	}

	before := walkSysroot()

	r := runner{
		dir: dir,
		env: []string{
			"WAV_PKG=" + name,
			"WAV_VERSION=" + version,
			"WAV_SYSROOT=" + sysroot(),
			"WAV_CACHE=" + cachedir(),
			"WAV_TMP=" + tmpdir(),
			"WAV_JOBS=" + jobs(),
			"WAV_SRC=" + wavSrc,
			"WAV_BUILD_DIR=" + extractDir,
			"PATH=" + os.Getenv("PATH"),
		},
	}

	if hasScript(dir, "build") {
		banner("building " + name)
		err := r.run("bash", dir+"/build")
		if err != nil {
			fatalln(err)
		}
	}

	if hasScript(dir, "install") {
		banner("installing " + name)
		err := r.run("bash", dir+"/install")
		if err != nil {
			fatalln(err)
		}
	}

	after := walkSysroot()
	manifest := diffFiles(after, before)

	err := recordInstall(name, version, dir)
	if err != nil {
		fatalln(err)
	}

	err = saveManifest(name, manifest)
	if err != nil {
		fatalln(err)
	}

	banner(name + " package compiled!")
}

func buildPkg(pkg string) {
	dir, ok := pkgDir(pkg)
	if !ok {
		die("package %s not found", pkg)
	}

	vars := recipeVars(dir)
	name := vars["name"]
	if name == "" {
		name = pkg
	}
	version := vars["version"]

	acquireSource(pkg)

	manifestFile := sysroot() + "/.wav/manifests/" + name
	data, err := os.ReadFile(manifestFile)
	if err != nil {
		die("no manifest found for %s (build may have failed)", name)
	}

	files := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(files) == 0 || (len(files) == 1 && files[0] == "") {
		die("no files to package for %s", name)
	}

	listFile := tmpdir() + "/" + name + "-filelist"
	os.WriteFile(listFile, []byte(strings.Join(files, "\n")), 0644)

	targetDir := pkgTargetDir(dir)
	if targetDir == "" {
		die("no pkgs repo directory found for %s", pkg)
	}

	os.MkdirAll(targetDir, 0755)
	tarball := targetDir + "/" + name + "-" + version + ".tar.zst"

	banner("packaging " + tarball)
	err = run("tar", "--zstd", "-cf", tarball, "--files-from", listFile, "-C", sysroot())
	if err != nil {
		fatalln(err)
	}

	banner(name + " prebuilt saved to " + tarball)
}

func acquirePrebuilt(pkg string) bool {
	cfg := loadConfig()
	for _, r := range cfg.Repos {
		rp := r.Path
		if !filepath.IsAbs(rp) {
			cwd, err := os.Getwd()
			if err != nil {
				continue
			}
			rp = cwd + "/" + rp
		}
		if !strings.Contains(rp, "/pkgs/") {
			continue
		}
		entries, err := os.ReadDir(rp)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if e.IsDir() {
				continue
			}
			if strings.HasPrefix(e.Name(), pkg+"-") &&
				(strings.HasSuffix(e.Name(), ".tar.zst") || strings.HasSuffix(e.Name(), ".tar.gz")) {
				rest := strings.TrimPrefix(e.Name(), pkg+"-")
				version := strings.TrimSuffix(rest, ".tar.zst")
				version = strings.TrimSuffix(version, ".tar.gz")

				banner("found prebuilt: " + e.Name())

				before := walkSysroot()
				err = run("tar", "-xf", rp+"/"+e.Name(), "-C", sysroot())
				if err != nil {
					fatalln(err)
				}
				after := walkSysroot()

				recordInstall(pkg, version, rp)
				saveManifest(pkg, diffFiles(after, before))

				banner(pkg + " package installed from prebuilt!")
				return true
			}
		}
	}
	return false
}

func pkgTargetDir(recipeDir string) string {
	cfg := loadConfig()
	recipeDir = filepath.Clean(recipeDir)

	for _, r := range cfg.Repos {
		rp := r.Path
		if !filepath.IsAbs(rp) {
			cwd, err := os.Getwd()
			if err != nil {
				continue
			}
			rp = cwd + "/" + rp
		}
		rp = filepath.Clean(rp)

		rel, err := filepath.Rel(rp, recipeDir)
		if err != nil || strings.HasPrefix(rel, "..") {
			continue
		}

		for _, p := range cfg.Repos {
			pp := p.Path
			if !strings.Contains(pp, "/pkgs/") {
				continue
			}
			if (strings.Contains(rp, "/recipes/core") && strings.Contains(pp, "/pkgs/core")) ||
				(strings.Contains(rp, "/recipes/extra") && strings.Contains(pp, "/pkgs/extra")) ||
				(strings.Contains(rp, "/recipes/desktop") && strings.Contains(pp, "/pkgs/desktop")) {
				if filepath.IsAbs(pp) {
					return pp
				}
				cwd, _ := os.Getwd()
				return cwd + "/" + pp
			}
		}
	}
	return ""
}

type runner struct {
	dir string
	env []string
}

func (r *runner) run(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Dir = r.dir
	cmd.Env = r.env
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func findActualSrcDir(parent string) string {
	entries, err := os.ReadDir(parent)
	if err != nil {
		return ""
	}
	dirs := []string{}
	for _, e := range entries {
		if e.IsDir() {
			dirs = append(dirs, e.Name())
		}
	}
	if len(dirs) != 1 {
		return ""
	}
	return parent + "/" + dirs[0]
}

func downloadSource(name, version, url string) error {
	cache := cachedir()
	err := os.MkdirAll(cache, 0755)
	if err != nil {
		return err
	}

	tarball := cache + "/" + tarballName(url)
	if _, err := os.Stat(tarball); err == nil {
		banner("source already cached: " + tarball)
		return nil
	}

	banner("downloading " + url)
	return run("wget", "-O", tarball, url)
}

func tarballName(url string) string {
	parts := strings.Split(url, "/")
	return parts[len(parts)-1]
}

func extractSource(url, dest string) error {
	tarball := cachedir() + "/" + tarballName(url)

	err := os.MkdirAll(dest, 0755)
	if err != nil {
		return err
	}

	banner("extracting " + tarball)

	if strings.HasSuffix(tarball, ".tar.xz") || strings.HasSuffix(tarball, ".txz") {
		return run("tar", "-xf", tarball, "-C", dest)
	}
	if strings.HasSuffix(tarball, ".tar.gz") || strings.HasSuffix(tarball, ".tgz") {
		return run("tar", "-xzf", tarball, "-C", dest)
	}
	if strings.HasSuffix(tarball, ".tar.bz2") {
		return run("tar", "-xjf", tarball, "-C", dest)
	}
	if strings.HasSuffix(tarball, ".tar.zst") {
		return run("tar", "--zstd", "-xf", tarball, "-C", dest)
	}
	return fmt.Errorf("unknown archive format: %s", tarball)
}

func hasScript(dir, name string) bool {
	info, err := os.Stat(dir + "/" + name)
	return err == nil && !info.IsDir()
}

func run(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func recordInstall(name, version, recipeDir string) error {
	dbdir := sysroot() + "/.wav/db"
	err := os.MkdirAll(dbdir, 0755)
	if err != nil {
		return err
	}

	entry := fmt.Sprintf("name=%s\nversion=%s\nrecipe=%s\n", name, version, recipeDir)
	return os.WriteFile(dbdir+"/"+name, []byte(entry), 0644)
}

func walkSysroot() map[string]bool {
	files := make(map[string]bool)
	root := sysroot()
	if _, err := os.Stat(root); os.IsNotExist(err) {
		return files
	}
	filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if path == root {
			return nil
		}
		rel, _ := filepath.Rel(root, path)
		files[rel] = true
		return nil
	})
	return files
}

func diffFiles(after, before map[string]bool) []string {
	var newFiles []string
	for f := range after {
		if !before[f] {
			newFiles = append(newFiles, f)
		}
	}
	return newFiles
}

func saveManifest(name string, files []string) error {
	manifestDir := sysroot() + "/.wav/manifests"
	err := os.MkdirAll(manifestDir, 0755)
	if err != nil {
		return err
	}

	var buf strings.Builder
	for _, f := range files {
		buf.WriteString(f)
		buf.WriteString("\n")
	}
	return os.WriteFile(manifestDir+"/"+name, []byte(buf.String()), 0644)
}
