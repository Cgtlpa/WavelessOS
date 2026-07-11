package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func annihilate(pkg string) {
	dbdir := sysroot() + "/.wav/db"
	dbFile := dbdir + "/" + pkg

	if _, err := os.Stat(dbFile); os.IsNotExist(err) {
		die("package %s is not installed", pkg)
	}

	banner("annihilating " + pkg)

	manifest := sysroot() + "/.wav/manifests/" + pkg
	dirsToClean := make(map[string]bool)

	if data, err := os.ReadFile(manifest); err == nil {
		lines := strings.Split(string(data), "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			full := sysroot() + "/" + line
			if err := os.Remove(full); err != nil {
				fmt.Fprintf(os.Stderr, "warning: could not remove %s: %v\n", full, err)
			} else {
				dir := filepath.Dir(full)
				for dir != "" && dir != sysroot() && dir != "/" {
					dirsToClean[dir] = true
					dir = filepath.Dir(dir)
				}
			}
		}
	}

	if err := os.Remove(dbFile); err != nil {
		fmt.Fprintf(os.Stderr, "warning: could not remove database entry %s: %v\n", dbFile, err)
	}
	if err := os.Remove(manifest); err != nil {
		fmt.Fprintf(os.Stderr, "warning: could not remove manifest %s: %v\n", manifest, err)
	}

	for dir := range dirsToClean {
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		if len(entries) == 0 {
			os.Remove(dir)
		}
	}

	banner(pkg + " successfully removed")
}
