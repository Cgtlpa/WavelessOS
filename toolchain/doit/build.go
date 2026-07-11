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

func recipeVars(file string) map[string]string {
	vars := make(map[string]string)
	f, err := os.Open(file)
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
		if strings.HasPrefix(line, "build()") || strings.HasPrefix(line, "install()") {
			break
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		val := strings.TrimSpace(parts[1])
		if strings.HasPrefix(val, "(") && strings.HasSuffix(val, ")") {
			val = strings.TrimPrefix(val, "(")
			val = strings.TrimSuffix(val, ")")
		}
		vars[strings.TrimSpace(parts[0])] = val
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

func extractFunc(file, funcName string) string {
	data, err := os.ReadFile(file)
	if err != nil {
		return ""
	}

	lines := strings.Split(string(data), "\n")
	inFunc := false
	braceCount := 0
	var body strings.Builder

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		if !inFunc {
			sig := funcName + "()"
			if strings.HasPrefix(trimmed, sig) {
				rest := strings.TrimSpace(strings.TrimPrefix(trimmed, sig))
				if rest == "{" || strings.HasPrefix(rest, "{") {
					inFunc = true
					braceCount = 1
					continue
				}
			}
			continue
		}

		for _, ch := range trimmed {
			if ch == '{' {
				braceCount++
			} else if ch == '}' {
				braceCount--
			}
		}

		if braceCount <= 0 {
			break
		}

		body.WriteString(line)
		body.WriteString("\n")
	}

	return body.String()
}

func runScript(env []string, script string) error {
	if strings.TrimSpace(script) == "" {
		return nil
	}
	tmp, err := os.CreateTemp("", "wav-*.sh")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	tmp.WriteString("#!/bin/sh\nset -e\n")
	tmp.WriteString(script)
	tmp.Close()
	cmd := exec.Command("sh", tmp.Name())
	cmd.Env = env
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
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

	var cachedPath string
	if source != "" {
		var err error
		cachedPath, err = downloadSource(name, version, source)
		if err != nil {
			fatalln(err)
		}
	}

	extractDir := tmpdir() + "/" + name
	if version != "" {
		extractDir += "-" + version
	}
	if err := os.MkdirAll(extractDir, 0755); err != nil {
		fatalln(err)
	}

	wavSrc := extractDir + "/src"
	if source != "" {
		err := extractSource(cachedPath, wavSrc)
		if err != nil {
			fatalln(err)
		}
	}

	actualSrc := findActualSrcDir(wavSrc)
	if actualSrc != "" {
		wavSrc = actualSrc
	}

	before := walkSysroot()

	env := []string{
		"WAV_PKG=" + name,
		"WAV_VERSION=" + version,
		"WAV_SYSROOT=" + sysroot(),
		"WAV_CACHE=" + cachedir(),
		"WAV_TMP=" + tmpdir(),
		"WAV_JOBS=" + jobs(),
		"WAV_SRC=" + wavSrc,
		"WAV_BUILD_DIR=" + extractDir,
		"PATH=" + os.Getenv("PATH"),
	}

	buildScript := extractFunc(dir, "build")
	if strings.TrimSpace(buildScript) != "" {
		banner("building " + name)
		err := runScript(env, buildScript)
		if err != nil {
			fatalln(err)
		}
	}

	installScript := extractFunc(dir, "install")
	if strings.TrimSpace(installScript) != "" {
		banner("installing " + name)
		err := runScript(env, installScript)
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
	if err := os.WriteFile(listFile, []byte(strings.Join(files, "\n")), 0644); err != nil {
		fatalln(err)
	}

	targetDir := pkgTargetDir(dir)
	if targetDir == "" {
		die("no pkgs repo directory found for %s", pkg)
	}

	if err := os.MkdirAll(targetDir, 0755); err != nil {
		fatalln(err)
	}
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

				if err := recordInstall(pkg, version, rp); err != nil {
					fmt.Fprintf(os.Stderr, "warning: failed to record install: %v\n", err)
				}
				if err := saveManifest(pkg, diffFiles(after, before)); err != nil {
					fmt.Fprintf(os.Stderr, "warning: failed to save manifest: %v\n", err)
				}

				banner(pkg + " package installed from prebuilt!")
				return true
			}
		}
	}
	return false
}

func pkgTargetDir(recipeFile string) string {
	cfg := loadConfig()
	recipeFile = filepath.Clean(recipeFile)

	category := ""
	for _, r := range cfg.Repos {
		rp := filepath.Clean(r.Path)
		rel, err := filepath.Rel(rp, filepath.Dir(recipeFile))
		if err != nil || strings.HasPrefix(rel, "..") {
			continue
		}
		category = r.Name
		break
	}

	targetMap := map[string]string{
		"core":    "pkgs/core",
		"extra":   "pkgs/extra",
		"desktop": "pkgs/desktop",
		"xfce":    "pkgs/desktop",
	}

	target, ok := targetMap[category]
	if !ok {
		return ""
	}
	for _, r := range cfg.Repos {
		if r.Path == target {
			return r.Path
		}
	}
	return ""
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

func downloadSource(name, version, url string) (string, error) {
	cache := cachedir()
	err := os.MkdirAll(cache, 0755)
	if err != nil {
		return "", err
	}

	tarball := cache + "/" + tarballName(url)
	if hasArchiveExt(tarball) {
		if _, err := os.Stat(tarball); err == nil {
			banner("source already cached: " + tarball)
			return tarball, nil
		}
	}

	banner("downloading " + url)
	tmpFile := cache + "/.download-tmp"
	err = run("wget", "-O", tmpFile, url)
	if err != nil {
		os.Remove(tmpFile)
		return "", err
	}

	if hasArchiveExt(tarball) {
		os.Rename(tmpFile, tarball)
		return tarball, nil
	}

	dst := detectAndRename(cache, tmpFile, name, version)
	return dst, nil
}

func tarballName(url string) string {
	clean := strings.SplitN(url, "?", 2)[0]
	parts := strings.Split(clean, "/")
	return parts[len(parts)-1]
}

func hasArchiveExt(name string) bool {
	for _, ext := range []string{".tar.xz", ".txz", ".tar.gz", ".tgz", ".tar.bz2", ".tar.zst", ".zip", ".deb", ".run"} {
		if strings.HasSuffix(name, ext) {
			return true
		}
	}
	return false
}

func detectFileType(path string) string {
	out, err := exec.Command("file", "-b", "--extension", path).Output()
	if err != nil {
		return ""
	}
	s := strings.TrimSpace(string(out))
	for _, t := range []struct {
		magic  string
		suffix string
	}{
		{"xz", ".tar.xz"},
		{"gzip", ".tar.gz"},
		{"bzip2", ".tar.bz2"},
		{"zstd", ".tar.zst"},
		{"zip", ".zip"},
		{"deb", ".deb"},
		{"executable", ".run"},
	} {
		if strings.Contains(s, t.magic) {
			return t.suffix
		}
	}
	return ""
}

func detectAndRename(cache, tmpFile, name, version string) string {
	ext := detectFileType(tmpFile)
	if ext == "" {
		ext = ".tar.gz"
	}
	final := name + "-" + version + ext
	dst := cache + "/" + final
	os.Rename(tmpFile, dst)
	return dst
}

func extractSource(tarball, dest string) error {
	run("rm", "-rf", dest)
	if err := os.MkdirAll(dest, 0755); err != nil {
		return err
	}
	if _, err := os.Stat(tarball); os.IsNotExist(err) {
		return fmt.Errorf("source not found: %s", tarball)
	}
	banner("extracting " + tarball)

	switch {
	case strings.HasSuffix(tarball, ".tar.xz") || strings.HasSuffix(tarball, ".txz"):
		return run("tar", "-xf", tarball, "-C", dest)
	case strings.HasSuffix(tarball, ".tar.gz") || strings.HasSuffix(tarball, ".tgz"):
		return run("tar", "-xzf", tarball, "-C", dest)
	case strings.HasSuffix(tarball, ".tar.bz2"):
		return run("tar", "-xjf", tarball, "-C", dest)
	case strings.HasSuffix(tarball, ".tar.zst"):
		return run("tar", "--zstd", "-xf", tarball, "-C", dest)
	case strings.HasSuffix(tarball, ".zip"):
		return run("unzip", "-o", tarball, "-d", dest)
	case strings.HasSuffix(tarball, ".deb") || strings.HasSuffix(tarball, ".run"):
		return run("cp", tarball, dest+"/")
	default:
		return fmt.Errorf("unknown archive format: %s", tarball)
	}
}

func run(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func recordInstall(name, version, recipeFile string) error {
	dbdir := sysroot() + "/.wav/db"
	err := os.MkdirAll(dbdir, 0755)
	if err != nil {
		return err
	}

	entry := fmt.Sprintf("name=%s\nversion=%s\nrecipe=%s\n", name, version, recipeFile)
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
