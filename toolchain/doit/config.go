package main

import (
	"os"
	"os/exec"
	"path/filepath"
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
	actualSrc := findActualSrcDir(extractDir + "/src")
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
	if err := cmd.Run(); err != nil {
		fatalln(err)
	}

	data, err := os.ReadFile(actualSrc + "/.config")
	if err != nil {
		banner("no .config found (did you save it?)")
		return
	}

	savedir := sysroot() + "/usr/src/" + name
	if err := os.MkdirAll(savedir, 0755); err == nil {
		os.WriteFile(savedir+"/.config", data, 0644)
		banner("config saved to " + savedir + "/.config")
	}

	configDir := filepath.Dir(dir)
	if err := os.WriteFile(configDir+"/config", data, 0644); err == nil {
		banner("config also saved to " + configDir + "/config")
	}
}
