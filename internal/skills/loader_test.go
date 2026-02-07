package skills

import (
	"os"
	"path/filepath"
	"sort"
	"testing"
)

func TestNewLoader(t *testing.T) {
	loader := NewLoader()

	if loader == nil {
		t.Fatal("expected non-nil loader")
	}
	if loader.skills == nil {
		t.Fatal("expected non-nil skills map")
	}
	if len(loader.skillDirs) < 2 {
		t.Fatalf("expected at least 2 skill dirs, got %d", len(loader.skillDirs))
	}
	// First two dirs should be the project-local defaults
	if loader.skillDirs[0] != "skills" {
		t.Errorf("expected first skill dir to be %q, got %q", "skills", loader.skillDirs[0])
	}
	if loader.skillDirs[1] != ".vecai/skills" {
		t.Errorf("expected second skill dir to be %q, got %q", ".vecai/skills", loader.skillDirs[1])
	}
}

func TestNewLoader_IncludesUserConfigDir(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip("cannot determine home directory")
	}

	loader := NewLoader()
	expected := filepath.Join(home, ".config", "vecai", "skills")

	found := false
	for _, dir := range loader.skillDirs {
		if dir == expected {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected skill dirs to include %q", expected)
	}
}

// makeTestSkillsDir creates a temp directory with skill markdown files and
// returns its path. The caller is responsible for cleanup.
func makeTestSkillsDir(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0644); err != nil {
			t.Fatalf("failed to write test skill file %s: %v", name, err)
		}
	}
	return dir
}

func TestLoadFromDir(t *testing.T) {
	dir := makeTestSkillsDir(t, map[string]string{
		"review.md": `---
name: review
description: Code review skill
triggers:
  - review
  - "/review.*code/"
tags:
  - code
---
Review the code carefully.
`,
		"deploy.md": `---
name: deploy
description: Deploy skill
triggers:
  - deploy
tags:
  - ops
---
Deploy the application.
`,
	})

	loader := &Loader{skills: make(map[string]*Skill)}
	loader.loadFromDir(dir)

	if len(loader.skills) != 2 {
		t.Fatalf("expected 2 skills, got %d", len(loader.skills))
	}

	review, ok := loader.skills["review"]
	if !ok {
		t.Fatal("expected skill 'review' to be loaded")
	}
	if review.Description != "Code review skill" {
		t.Errorf("expected description %q, got %q", "Code review skill", review.Description)
	}
	if review.Content != "Review the code carefully." {
		t.Errorf("unexpected content: %q", review.Content)
	}
	if len(review.Triggers) != 2 {
		t.Errorf("expected 2 triggers, got %d", len(review.Triggers))
	}
	if len(review.Tags) != 1 || review.Tags[0] != "code" {
		t.Errorf("unexpected tags: %v", review.Tags)
	}

	deploy, ok := loader.skills["deploy"]
	if !ok {
		t.Fatal("expected skill 'deploy' to be loaded")
	}
	if deploy.Description != "Deploy skill" {
		t.Errorf("expected description %q, got %q", "Deploy skill", deploy.Description)
	}
}

func TestLoadFromDir_SkipsNonMarkdown(t *testing.T) {
	dir := makeTestSkillsDir(t, map[string]string{
		"readme.txt": "not a skill",
		"config.yaml": "also not a skill",
		"actual.md": `---
name: actual
triggers:
  - actual
---
Content here.
`,
	})

	loader := &Loader{skills: make(map[string]*Skill)}
	loader.loadFromDir(dir)

	if len(loader.skills) != 1 {
		t.Fatalf("expected 1 skill, got %d", len(loader.skills))
	}
	if _, ok := loader.skills["actual"]; !ok {
		t.Error("expected skill 'actual' to be loaded")
	}
}

func TestLoadFromDir_SkipsDirectories(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "subdir.md"), 0755); err != nil {
		t.Fatal(err)
	}

	loader := &Loader{skills: make(map[string]*Skill)}
	loader.loadFromDir(dir)

	if len(loader.skills) != 0 {
		t.Fatalf("expected 0 skills, got %d", len(loader.skills))
	}
}

func TestLoadFromDir_MissingDir(t *testing.T) {
	loader := &Loader{skills: make(map[string]*Skill)}
	loader.loadFromDir("/nonexistent/path/that/should/not/exist")

	if len(loader.skills) != 0 {
		t.Fatalf("expected 0 skills for missing dir, got %d", len(loader.skills))
	}
}

func TestLoadFromDir_NoFrontmatter(t *testing.T) {
	dir := makeTestSkillsDir(t, map[string]string{
		"plain.md": "Just plain markdown content without frontmatter.",
	})

	loader := &Loader{skills: make(map[string]*Skill)}
	loader.loadFromDir(dir)

	if len(loader.skills) != 1 {
		t.Fatalf("expected 1 skill, got %d", len(loader.skills))
	}

	skill, ok := loader.skills["plain"]
	if !ok {
		t.Fatal("expected skill 'plain' (filename without extension) to be loaded")
	}
	if skill.Content != "Just plain markdown content without frontmatter." {
		t.Errorf("unexpected content: %q", skill.Content)
	}
}

func TestLoadFromDir_NameFallsBackToFilename(t *testing.T) {
	dir := makeTestSkillsDir(t, map[string]string{
		"my-skill.md": `---
description: A skill without a name field
triggers:
  - hello
---
Body text.
`,
	})

	loader := &Loader{skills: make(map[string]*Skill)}
	loader.loadFromDir(dir)

	if _, ok := loader.skills["my-skill"]; !ok {
		keys := make([]string, 0, len(loader.skills))
		for k := range loader.skills {
			keys = append(keys, k)
		}
		t.Fatalf("expected skill key 'my-skill', got keys: %v", keys)
	}
}

func TestMatch_PlainTrigger(t *testing.T) {
	loader := &Loader{
		skills: map[string]*Skill{
			"greet": {
				Name:     "greet",
				Triggers: []string{"hello", "hi there"},
			},
		},
	}

	tests := []struct {
		input string
		want  bool
	}{
		{"say hello to the world", true},
		{"HI THERE friend", true},
		{"goodbye", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := loader.Match(tt.input)
			if tt.want && got == nil {
				t.Errorf("Match(%q) = nil, want skill 'greet'", tt.input)
			}
			if !tt.want && got != nil {
				t.Errorf("Match(%q) = %v, want nil", tt.input, got.Name)
			}
		})
	}
}

func TestMatch_RegexTrigger(t *testing.T) {
	loader := &Loader{
		skills: map[string]*Skill{
			"refactor": {
				Name:     "refactor",
				Triggers: []string{"/refactor.*code/", "/^optimize/"},
			},
		},
	}

	tests := []struct {
		input string
		want  bool
	}{
		{"please refactor this code", true},
		{"refactor the code base", true},
		{"optimize performance", true},
		{"let us refactor", false},  // "refactor.*code" not matched
		{"do not optimize", false},  // "^optimize" anchor fails
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := loader.Match(tt.input)
			if tt.want && got == nil {
				t.Errorf("Match(%q) = nil, want skill 'refactor'", tt.input)
			}
			if !tt.want && got != nil {
				t.Errorf("Match(%q) = %v, want nil", tt.input, got.Name)
			}
		})
	}
}

func TestMatch_NoSkills(t *testing.T) {
	loader := &Loader{skills: make(map[string]*Skill)}

	if got := loader.Match("anything"); got != nil {
		t.Errorf("Match on empty loader = %v, want nil", got.Name)
	}
}

func TestGet(t *testing.T) {
	loader := &Loader{
		skills: map[string]*Skill{
			"test": {Name: "test", Description: "test skill"},
		},
	}

	skill, ok := loader.Get("test")
	if !ok {
		t.Fatal("expected to find skill 'test'")
	}
	if skill.Name != "test" {
		t.Errorf("expected name 'test', got %q", skill.Name)
	}

	_, ok = loader.Get("nonexistent")
	if ok {
		t.Error("expected not to find skill 'nonexistent'")
	}
}

func TestList(t *testing.T) {
	loader := &Loader{
		skills: map[string]*Skill{
			"alpha": {Name: "alpha"},
			"beta":  {Name: "beta"},
			"gamma": {Name: "gamma"},
		},
	}

	list := loader.List()
	if len(list) != 3 {
		t.Fatalf("expected 3 skills, got %d", len(list))
	}

	names := make([]string, len(list))
	for i, s := range list {
		names[i] = s.Name
	}
	sort.Strings(names)

	expected := []string{"alpha", "beta", "gamma"}
	for i, name := range names {
		if name != expected[i] {
			t.Errorf("expected name %q at index %d, got %q", expected[i], i, name)
		}
	}
}

func TestList_Empty(t *testing.T) {
	loader := &Loader{skills: make(map[string]*Skill)}

	list := loader.List()
	if len(list) != 0 {
		t.Fatalf("expected 0 skills, got %d", len(list))
	}
}

func TestAddDir(t *testing.T) {
	dir := makeTestSkillsDir(t, map[string]string{
		"extra.md": `---
name: extra
description: Added via AddDir
triggers:
  - extra
---
Extra skill content.
`,
	})

	loader := &Loader{skills: make(map[string]*Skill)}
	loader.AddDir(dir)

	if len(loader.skillDirs) != 1 {
		t.Fatalf("expected 1 skill dir, got %d", len(loader.skillDirs))
	}

	skill, ok := loader.skills["extra"]
	if !ok {
		t.Fatal("expected skill 'extra' to be loaded after AddDir")
	}
	if skill.Description != "Added via AddDir" {
		t.Errorf("unexpected description: %q", skill.Description)
	}
}

func TestReload(t *testing.T) {
	dir := makeTestSkillsDir(t, map[string]string{
		"initial.md": `---
name: initial
triggers:
  - init
---
Initial content.
`,
	})

	loader := &Loader{
		skills:    make(map[string]*Skill),
		skillDirs: []string{dir},
	}
	loader.loadSkills()

	if len(loader.skills) != 1 {
		t.Fatalf("expected 1 skill before reload, got %d", len(loader.skills))
	}

	// Add another file to the directory
	err := os.WriteFile(filepath.Join(dir, "second.md"), []byte(`---
name: second
triggers:
  - sec
---
Second content.
`), 0644)
	if err != nil {
		t.Fatal(err)
	}

	loader.Reload()

	if len(loader.skills) != 2 {
		t.Fatalf("expected 2 skills after reload, got %d", len(loader.skills))
	}
}

func TestSkill_GetPrompt(t *testing.T) {
	skill := &Skill{Content: "Do the thing."}
	if got := skill.GetPrompt(); got != "Do the thing." {
		t.Errorf("GetPrompt() = %q, want %q", got, "Do the thing.")
	}
}

func TestSkill_Matches(t *testing.T) {
	skill := &Skill{
		Triggers: []string{"deploy", "/^run tests/"},
	}

	tests := []struct {
		input string
		want  bool
	}{
		{"please deploy now", true},
		{"DEPLOY this", true},
		{"run tests please", true},
		{"do not run tests", false}, // anchor prevents match
		{"nothing relevant", false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			if got := skill.Matches(tt.input); got != tt.want {
				t.Errorf("Matches(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestLoadSkill_InvalidFrontmatter(t *testing.T) {
	dir := makeTestSkillsDir(t, map[string]string{
		"bad.md": `---
name: [invalid yaml
  this is broken
---
Content.
`,
	})

	loader := &Loader{skills: make(map[string]*Skill)}
	loader.loadFromDir(dir)

	// Invalid frontmatter should be skipped (warning printed to stderr)
	if len(loader.skills) != 0 {
		t.Fatalf("expected 0 skills for invalid frontmatter, got %d", len(loader.skills))
	}
}
