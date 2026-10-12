//go:build ignore

// Fetches the newest upstream lists and writes the files this package embeds. Run from api/:
//
//	go run ./internal/analytics/lists/update.go
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"gopkg.in/yaml.v3"
)

const dir = "internal/analytics/lists"

func get(url string) []byte {
	res, err := http.Get(url)
	if err != nil {
		log.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		log.Fatalf("%s: %s", url, res.Status)
	}
	b, err := io.ReadAll(res.Body)
	if err != nil {
		log.Fatal(err)
	}
	return b
}

func head(repo string) string {
	var c struct {
		SHA string `json:"sha"`
	}
	if err := json.Unmarshal(get("https://api.github.com/repos/"+repo+"/commits/HEAD"), &c); err != nil || c.SHA == "" {
		log.Fatalf("%s: no commit: %v", repo, err)
	}
	return c.SHA
}

func raw(repo, sha, path string) []byte {
	return get("https://raw.githubusercontent.com/" + repo + "/" + sha + "/" + path)
}

func write(name, header string, lines []string) {
	out := header + strings.Join(lines, "\n") + "\n"
	if err := os.WriteFile(filepath.Join(dir, name), []byte(out), 0o644); err != nil {
		log.Fatal(err)
	}
}

func main() {
	repo := "monperrus/crawler-user-agents"
	sha := head(repo)
	var crawlers []struct {
		Pattern string `json:"pattern"`
	}
	if err := json.Unmarshal(raw(repo, sha, "crawler-user-agents.json"), &crawlers); err != nil {
		log.Fatal(err)
	}
	var patterns []string
	for _, c := range crawlers {
		patterns = append(patterns, c.Pattern)
	}
	write("crawlers.txt", fmt.Sprintf("# https://github.com/%s at %s, MIT License (see api/THIRD_PARTY_NOTICES.md); the patterns only.\n", repo, sha), patterns)

	repo = "matomo-org/referrer-spam-list"
	sha = head(repo)
	var spam []string
	for _, l := range strings.Split(string(raw(repo, sha, "spammers.txt")), "\n") {
		if l = strings.ToLower(strings.TrimSpace(l)); l != "" {
			spam = append(spam, l)
		}
	}
	slices.Sort(spam)
	write("spammers.txt", fmt.Sprintf("# https://github.com/%s at %s, public domain (CC0-1.0).\n", repo, sha), slices.Compact(spam))

	repo = "matomo-org/searchengine-and-social-list"
	sha = head(repo)
	var rows []string
	var engines map[string][]struct {
		URLs []string `yaml:"urls"`
	}
	if err := yaml.Unmarshal(raw(repo, sha, "SearchEngines.yml"), &engines); err != nil {
		log.Fatal(err)
	}
	for name, defs := range engines {
		for _, d := range defs {
			for _, u := range d.URLs {
				rows = append(rows, "search\t"+name+"\t"+strings.ToLower(u))
			}
		}
	}
	for file, kind := range map[string]string{"Socials.yml": "social", "AIAssistants.yml": "ai"} {
		var m map[string][]string
		if err := yaml.Unmarshal(raw(repo, sha, file), &m); err != nil {
			log.Fatal(err)
		}
		for name, urls := range m {
			for _, u := range urls {
				rows = append(rows, kind+"\t"+name+"\t"+strings.ToLower(u))
			}
		}
	}
	slices.Sort(rows)
	write("sources.tsv", fmt.Sprintf("# https://github.com/%s at %s, CC0-1.0: kind, name, host (with {} for any label) and an optional path.\n", repo, sha), slices.Compact(rows))
}
