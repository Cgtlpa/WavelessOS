package main

import (
	"fmt"
	"os"
	"strings"
)

func wave() {
	if len(os.Args) < 4 {
		die("usage: doit wave <acquire(Install)|annihilate(Remove)|find(Search Package)> <package|query>")
	}

	action := os.Args[2]
	arg := os.Args[3]

	switch action {
	case "acquire":
		acquire(arg)
	case "annihilate":
		annihilate(arg)
	case "find":
		search(arg)
	default:
		die("unknown action: %s", action)
	}
}

func pkgDir(name string) (string, bool) {
	dirs := []string{
		"recipes/core/" + name,
		"recipes/extra/" + name,
		"recipes/desktop/" + name,
		"pkgs/core/" + name,
		"pkgs/extra/" + name,
	}

	cwd, err := os.Getwd()
	if err != nil {
		return "", false
	}

	for _, d := range dirs {
		path := cwd + "/" + d
		if info, err := os.Stat(path); err == nil && info.IsDir() {
			return path, true
		}
	}
	return "", false
}

func banner(msg string) {
	fmt.Println("---", msg)
}

func search(query string) {
	cwd, err := os.Getwd()
	if err != nil {
		die("cant get working dir")
	}

	roots := []string{
		cwd + "/recipes/core",
		cwd + "/recipes/extra",
		cwd + "/recipes/desktop",
		cwd + "/pkgs/core",
		cwd + "/pkgs/extra",
	}

	found := 0
	for _, root := range roots {
		entries, err := os.ReadDir(root)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if !e.IsDir() {
				continue
			}
			name := e.Name()
			if query == "" || strings.Contains(strings.ToLower(name), strings.ToLower(query)) {
				recipePath := root + "/" + name + "/recipe"
				version := ""
				if data, err := os.ReadFile(recipePath); err == nil {
					for _, line := range strings.Split(string(data), "\n") {
						if strings.HasPrefix(line, "version=") {
							version = strings.TrimPrefix(line, "version=")
							break
						}
					}
				}
				where := "recipes/"
				if strings.Contains(root, "/pkgs/") {
					where = "pkgs/"
				}
				if version != "" {
					fmt.Printf("  %s (%s) [%s]\n", name, version, where)
				} else {
					fmt.Printf("  %s [%s]\n", name, where)
				}
				found++
			}
		}
	}

	if found == 0 {
		fmt.Println("  no packages found for \"" + query + "\"")
	} else {
		fmt.Printf("  %d package(s) were found\n", found)
	}
}
