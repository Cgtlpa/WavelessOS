package main

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
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

	r := runner{
		dir: dir,
		env: []string{
			"WAV_PKG=" + name,
			"WAV_VERSION=" + version,
			"WAV_SYSROOT=" + sysroot(),
			"WAV_CACHE=" + cachedir(),
			"WAV_TMP=" + tmpdir(),
			"WAV_JOBS=" + jobs(),
			"WAV_SRC=" + srcDir(name, version),
			"WAV_BUILD_DIR=" + buildDir(name, version),
			"PATH=" + os.Getenv("PATH"),
		},
	}

	if source != "" {
		err := extractSource(name, version, source)
		if err != nil {
			fatalln(err)
		}
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

	err := recordInstall(name, version, dir)
	if err != nil {
		fatalln(err)
	}

	banner(name + " package compiled!")
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

func srcDir(name, version string) string {
	return buildDir(name, version) + "/src"
}

func buildDir(name, version string) string {
	d := tmpdir() + "/" + name
	if version != "" {
		d += "-" + version
	}
	return d
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

func extractSource(name, version, url string) error {
	tarball := cachedir() + "/" + tarballName(url)
	dest := buildDir(name, version)

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
