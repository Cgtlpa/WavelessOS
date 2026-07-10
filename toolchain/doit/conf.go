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
	cfg := &Config{
		Repos: defaultRepos(),
	}

	candidates := []string{
		"wave.conf",
		sysroot() + "/etc/wave.conf",
		"/etc/wave.conf",
	}

	for _, path := range candidates {
		f, err := os.Open(path)
		if err != nil {
			continue
		}
		defer f.Close()

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

			if strings.HasPrefix(line, "Path") || strings.HasPrefix(line, "path") {
				val := strings.TrimSpace(line[4:])
				if strings.HasPrefix(val, "=") {
					val = strings.TrimSpace(val[1:])
				}
				if val != "" {
					cur.Path = val
				}
			}
		}
		if cur != nil && cur.Path != "" {
			repos = append(repos, *cur)
		}

		if len(repos) > 0 {
			cfg.Repos = repos
		}
		break
	}

	return cfg
}

func defaultRepos() []Repo {
	return []Repo{
		{Name: "core", Path: "recipes/core"},
		{Name: "extra", Path: "recipes/extra"},
		{Name: "desktop", Path: "recipes/desktop"},
		{Name: "pkgs-core", Path: "pkgs/core"},
		{Name: "pkgs-extra", Path: "pkgs/extra"},
		{Name: "pkgs-desktop", Path: "pkgs/desktop"},
	}
}

func (c *Config) repoPaths() []string {
	cwd, err := os.Getwd()
	if err != nil {
		var out []string
		for _, r := range c.Repos {
			out = append(out, r.Path)
		}
		return out
	}
	var out []string
	for _, r := range c.Repos {
		if filepath.IsAbs(r.Path) {
			out = append(out, r.Path)
		} else {
			out = append(out, cwd+"/"+r.Path)
		}
	}
	return out
}

func (c *Config) repoLabel(path string) string {
	for _, r := range c.Repos {
		rp := r.Path
		if !filepath.IsAbs(rp) {
			cwd, _ := os.Getwd()
			rp = cwd + "/" + rp
		}
		if strings.HasPrefix(path, rp) {
			return r.Name
		}
	}
	return "unknown"
}
