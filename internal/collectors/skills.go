package collectors

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/SuperMarioYL/agentmanifest/internal/manifest"
	"gopkg.in/yaml.v3"
)

// collectSkills scans third-party skill directories (SKILL.md format) under
// the user home and the project. Skill directories are read as data only —
// nothing inside a skill is executed or sandboxed.
func (s *scanner) collectSkills() {
	roots := []struct{ label, dir string }{
		{"~/.claude/skills", filepath.Join(s.opts.Home, ".claude", "skills")},
		{"<project>/.claude/skills", filepath.Join(s.opts.Project, ".claude", "skills")},
	}
	for _, root := range roots {
		entries, err := os.ReadDir(root.dir)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if !e.IsDir() {
				continue
			}
			dir := filepath.Join(root.dir, e.Name())
			if _, err := os.Stat(filepath.Join(dir, "SKILL.md")); err != nil {
				continue
			}
			s.collectSkill(root.label, dir)
		}
	}
}

// skillFrontmatter is the subset of SKILL.md YAML frontmatter the inventory uses.
type skillFrontmatter struct {
	Name        string `yaml:"name"`
	Description string `yaml:"description"`
}

func (s *scanner) collectSkill(rootLabel, dir string) {
	name := filepath.Base(dir)
	fmPath := filepath.Join(dir, "SKILL.md")
	if fm, ok := readCap(fmPath, maxSkillFileBytes); ok {
		if parsed := parseSkillFrontmatter(fm); parsed.Name != "" {
			name = parsed.Name
		}
	}

	t := manifest.Tool{
		Name:        name,
		Kind:        manifest.KindSkill,
		InstallPath: rootLabel + "/" + filepath.Base(dir),
	}
	files := 0
	_ = filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		files++
		if files > maxFilesPerDir {
			return filepath.SkipAll
		}
		if content, ok := readCap(path, maxSkillFileBytes); ok {
			src := filepath.Base(dir) + "/" + filepath.Base(path)
			addCredentials(&t, src, content)
			addDeclaredEgress(&t, src, content)
		}
		return nil
	})
	s.appendTool(t)
}

// parseSkillFrontmatter reads the name/description fields from a SKILL.md
// YAML frontmatter block; malformed frontmatter falls back to the directory name.
func parseSkillFrontmatter(content string) skillFrontmatter {
	lines := strings.Split(content, "\n")
	if len(lines) == 0 || strings.TrimSpace(lines[0]) != "---" {
		return skillFrontmatter{}
	}
	end := -1
	for i := 1; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == "---" {
			end = i
			break
		}
	}
	if end < 0 {
		return skillFrontmatter{}
	}
	var fm skillFrontmatter
	if yaml.Unmarshal([]byte(strings.Join(lines[1:end], "\n")), &fm) != nil {
		return skillFrontmatter{}
	}
	return fm
}
