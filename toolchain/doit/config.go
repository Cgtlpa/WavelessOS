package main

import (
	"os"
	"os/exec"
)

func config(pkg string) {
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

	extractDir := tmpdir() + "/" + name
	if version != "" {
		extractDir += "-" + version
	}

	src := extractDir + "/src"
	actualSrc := findActualSrcDir(src)
	if actualSrc == "" {
		die("source not extracted yet, run 'doit wave acquire %s' first", pkg)
	}

	cmd := exec.Command("make", "menuconfig")
	cmd.Dir = actualSrc
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = append(os.Environ(),
		"WAV_PKG="+name,
		"WAV_VERSION="+version,
		"WAV_SYSROOT="+sysroot(),
	)
	err := cmd.Run()
	if err != nil {
		fatalln(err)
	}

	srcfile := actualSrc + "/.config"
	data, err := os.ReadFile(srcfile)
	if err != nil {
		banner("no .config found (did you save it?)")
		return
	}

	savedir := sysroot() + "/usr/src/" + name
	os.MkdirAll(savedir, 0755)
	os.WriteFile(savedir+"/.config", data, 0644)
	banner("config saved to " + savedir + "/.config")

	os.WriteFile(dir+"/config", data, 0644)
	banner("config also saved to " + dir + "/config")
}
