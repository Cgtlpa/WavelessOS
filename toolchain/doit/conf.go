package main

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
)

type Repo struct {
	Name string
	Path string
}

type Config struct {
	Repos []Repo
}

func loadConfig() *Config {
	cfg := &Config{Repos: defaultRepos()}

	candidates := []string{
		sysroot() + "/etc/wave.conf",
		"/etc/wave.conf",
	}
	if cwd, err := os.Getwd(); err == nil {
		for dir := cwd; dir != filepath.Dir(dir); dir = filepath.Dir(dir) {
			candidates = append([]string{dir + "/wave.conf"}, candidates...)
		}
	}

	for _, path := range candidates {
		f, err := os.Open(path)
		if err != nil {
			continue
		}
		repos := parseConf(f)
		f.Close()
		if len(repos) == 0 {
			continue
		}
		cfg.Repos = repos
		configDir := filepath.Dir(path)
		for i := range cfg.Repos {
			if rp := cfg.Repos[i].Path; !filepath.IsAbs(rp) {
				cfg.Repos[i].Path = filepath.Clean(configDir + "/" + rp)
			}
		}
		break
	}
	return cfg
}

func parseConf(f *os.File) []Repo {
	var repos []Repo
	var cur *Repo
	scan := bufio.NewScanner(f)
	for scan.Scan() {
		line := strings.TrimSpace(scan.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			if cur != nil && cur.Path != "" {
				repos = append(repos, *cur)
			}
			cur = &Repo{Name: line[1 : len(line)-1]}
			continue
		}
		if cur == nil {
			continue
		}
		if strings.HasPrefix(strings.ToLower(line), "path") {
			parts := strings.SplitN(line, "=", 2)
			if len(parts) == 2 {
				if val := strings.TrimSpace(parts[1]); val != "" {
					cur.Path = val
				}
			}
		}
	}
	if cur != nil && cur.Path != "" {
		repos = append(repos, *cur)
	}
	return repos
}

func defaultRepos() []Repo {
	return []Repo{
		{Name: "core", Path: "recipes/core"},
		{Name: "extra", Path: "recipes/extra"},
		{Name: "desktop", Path: "recipes/desktop"},
		{Name: "xfce", Path: "recipes/xfce"},
		{Name: "pkgs-core", Path: "pkgs/core"},
		{Name: "pkgs-extra", Path: "pkgs/extra"},
		{Name: "pkgs-desktop", Path: "pkgs/desktop"},
	}
}


