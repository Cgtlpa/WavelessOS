package main

import (
	"os"
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
	if data, err := os.ReadFile(manifest); err == nil {
		lines := strings.Split(string(data), "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			full := sysroot() + "/" + line
			os.Remove(full)
		}
	}

	os.Remove(dbFile)
	os.Remove(manifest)

	cleanupEmptyDirs(sysroot())

	banner(pkg + " successfully removed")
}

func cleanupEmptyDirs(root string) {
	dirs := []string{
		root + "/bin",
		root + "/sbin",
		root + "/lib",
		root + "/lib64",
		root + "/usr/bin",
		root + "/usr/sbin",
		root + "/usr/lib",
		root + "/usr/lib64",
		root + "/usr/include",
		root + "/usr/share",
		root + "/etc",
	}
	for _, d := range dirs {
		entries, err := os.ReadDir(d)
		if err != nil {
			continue
		}
		if len(entries) == 0 {
			os.Remove(d)
		}
	}
}
