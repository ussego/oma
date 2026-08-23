package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Skills ship as discovery stubs in skills/ (installers copy those into agent
// skill dirs). Full content lives only in skill-data/ and is read by the CLI
// on demand — one source of truth, updated with the package, no network.

// skillDataDir resolves the private skill-data directory: extracted from the
// binary's embedded assets into the user cache dir on first use.
func skillDataDir() string {
	dir, err := extractSkillData()
	if err != nil {
		return ""
	}
	return dir
}

// unknownSkill is returned by `skills get <name>` for a name that isn't
// present; main prints its message verbatim and exits non-zero.
type unknownSkill struct{ name string }

func (e unknownSkill) Error() string { return "Unknown skill: " + e.name }

func runSkills(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: oma skills list | oma skills get <name> [--full] | oma skills get --all [--full]")
	}
	switch args[0] {
	case "list":
		return skillsList()
	case "get":
		return skillsGet(args[1:])
	default:
		return fmt.Errorf("unknown skills subcommand: %s", args[0])
	}
}

func skillsList() error {
	dir := skillDataDir()
	names, err := skillNames(dir)
	if err != nil {
		return err
	}
	for _, name := range names {
		meta, err := readSkillFrontmatter(dir, name)
		if err != nil {
			return err
		}
		fmt.Printf("%s\t%s\n", name, meta.description)
	}
	return nil
}

func skillsGet(args []string) error {
	name := ""
	all := false
	full := false
	for _, a := range args {
		switch a {
		case "--all":
			all = true
		case "--full":
			full = true
		default:
			name = a
		}
	}

	dir := skillDataDir()
	if all {
		names, err := skillNames(dir)
		if err != nil {
			return err
		}
		for i, n := range names {
			if i > 0 {
				fmt.Println("---")
			}
			if err := printSkill(dir, n, full); err != nil {
				return err
			}
		}
		return nil
	}

	if name == "" {
		return fmt.Errorf("usage: oma skills get <name> [--full] | oma skills get --all [--full]")
	}
	if !skillExists(dir, name) {
		return unknownSkill{name: name}
	}
	return printSkill(dir, name, full)
}

func printSkill(dir, name string, full bool) error {
	body, err := os.ReadFile(filepath.Join(dir, name, "SKILL.md"))
	if err != nil {
		return err
	}
	fmt.Print(string(body))
	if !full {
		return nil
	}

	refDir := filepath.Join(dir, name, "references")
	entries, err := os.ReadDir(refDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	var files []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".md") {
			files = append(files, e.Name())
		}
	}
	sort.Strings(files)
	for _, f := range files {
		content, err := os.ReadFile(filepath.Join(refDir, f))
		if err != nil {
			return err
		}
		fmt.Printf("\n# references/%s\n\n%s", f, content)
	}
	return nil
}

func skillNames(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var names []string
	for _, e := range entries {
		if e.IsDir() {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)
	return names, nil
}

func skillExists(dir, name string) bool {
	_, err := os.Stat(filepath.Join(dir, name, "SKILL.md"))
	return err == nil
}

type skillMeta struct {
	name        string
	description string
}

func readSkillFrontmatter(dir, name string) (skillMeta, error) {
	content, err := os.ReadFile(filepath.Join(dir, name, "SKILL.md"))
	if err != nil {
		return skillMeta{}, err
	}
	return parseFrontmatter(string(content)), nil
}

// parseFrontmatter extracts the two YAML frontmatter fields (name, description)
// from a SKILL.md body. Deliberately a two-line parser, not a YAML library.
func parseFrontmatter(content string) skillMeta {
	var m skillMeta
	if !strings.HasPrefix(content, "---\n") {
		return m
	}
	rest := content[len("---\n"):]
	end := strings.Index(rest, "\n---")
	if end < 0 {
		return m
	}
	for _, line := range strings.Split(rest[:end], "\n") {
		line = strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(line, "name:"):
			m.name = strings.TrimSpace(strings.TrimPrefix(line, "name:"))
		case strings.HasPrefix(line, "description:"):
			m.description = strings.TrimSpace(strings.TrimPrefix(line, "description:"))
		}
	}
	return m
}
