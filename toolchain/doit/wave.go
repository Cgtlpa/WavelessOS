package main

import (
	"fmt"
	"os"
	"strings"
)

func wave() {
	if len(os.Args) < 3 {
		die("usage: doit wave <acquire|build|annihilate|find|config> <package|query>")
	}

	action := os.Args[2]
	arg := ""
	if len(os.Args) >= 4 {
		arg = os.Args[3]
	}

	needsPkg := action == "acquire" || action == "annihilate" || action == "build" || action == "config"
	if needsPkg && arg == "" {
		die("usage: doit wave %s <package>", action)
	}

	switch action {
	case "acquire":
		acquire(arg)
	case "annihilate":
		annihilate(arg)
	case "find":
		search(arg)
	case "build":
		buildPkg(arg)
	case "config":
		config(arg)
	default:
		die("unknown action: %s", action)
	}
}

func pkgDir(name string) (string, bool) {
	cfg := loadConfig()
	for _, r := range cfg.Repos {
		path := r.Path + "/" + name
		if info, err := os.Stat(path); err == nil && !info.IsDir() {
			return path, true
		}
	}
	return "", false
}

func banner(msg string) {
	fmt.Println("---", msg)
}

func search(query string) {
	cfg := loadConfig()
	found := 0
	for _, r := range cfg.Repos {
		entries, err := os.ReadDir(r.Path)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if e.IsDir() {
				continue
			}
			name := e.Name()
			if query != "" && !strings.Contains(strings.ToLower(name), strings.ToLower(query)) {
				continue
			}
			version := ""
			if data, err := os.ReadFile(r.Path + "/" + name); err == nil {
				for _, line := range strings.Split(string(data), "\n") {
					if strings.HasPrefix(line, "version=") {
						version = strings.TrimPrefix(line, "version=")
						break
					}
				}
			}
			if version != "" {
				fmt.Printf("  %s (%s) [%s]\n", name, version, r.Name)
			} else {
				fmt.Printf("  %s [%s]\n", name, r.Name)
			}
			found++
		}
	}
	if found == 0 {
		fmt.Println("  no packages found for \"" + query + "\"")
	} else {
		fmt.Printf("  %d package(s) were found\n", found)
	}
}
