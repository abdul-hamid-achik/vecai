package skills

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

// Skill represents a loaded skill
type Skill struct {
	Name        string   `yaml:"name"`
	Description string   `yaml:"description"`
	Triggers    []string `yaml:"triggers"`
	Tags        []string `yaml:"tags"`
	Content     string   `yaml:"-"` // The markdown content after frontmatter
}

// Loader loads and matches skills
type Loader struct {
	skills    map[string]*Skill
	skillDirs []string
}

// NewLoader creates a new skill loader
func NewLoader() *Loader {
	loader := &Loader{
		skills: make(map[string]*Skill),
		skillDirs: []string{
			"skills",
			".vecai/skills",
		},
	}

	// Add user skills directory
	if home, err := os.UserHomeDir(); err == nil {
		loader.skillDirs = append(loader.skillDirs, filepath.Join(home, ".config", "vecai", "skills"))
	}

	// Load skills on creation
	loader.loadSkills()

	return loader
}

// loadSkills loads all skills from skill directories
func (l *Loader) loadSkills() {
	for _, dir := range l.skillDirs {
		l.loadFromDir(dir)
	}
}

// loadFromDir loads skills from a directory
func (l *Loader) loadFromDir(dir string) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return // Directory doesn't exist, skip
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		name := entry.Name()
		if !strings.HasSuffix(name, ".md") {
			continue
		}

		path := filepath.Join(dir, name)
		skill, err := l.loadSkill(path)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to load skill %s: %v\n", path, err)
			continue
		}

		// Use filename without extension as key if no name specified
		if skill.Name == "" {
			skill.Name = strings.TrimSuffix(name, ".md")
		}

		l.skills[skill.Name] = skill
	}
}

// loadSkill loads a single skill from a file
func (l *Loader) loadSkill(path string) (*Skill, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	skill := &Skill{}

	// Parse frontmatter if present
	text := string(content)
	if strings.HasPrefix(text, "---") {
		parts := strings.SplitN(text[3:], "---", 2)
		if len(parts) == 2 {
			if err := yaml.Unmarshal([]byte(parts[0]), skill); err != nil {
				return nil, fmt.Errorf("invalid frontmatter: %w", err)
			}
			skill.Content = strings.TrimSpace(parts[1])
		}
	} else {
		skill.Content = text
	}

	return skill, nil
}

// Match finds a skill that matches the given input
func (l *Loader) Match(input string) *Skill {
	input = strings.ToLower(input)

	for _, skill := range l.skills {
		for _, trigger := range skill.Triggers {
			// Check for regex trigger
			if strings.HasPrefix(trigger, "/") && strings.HasSuffix(trigger, "/") {
				pattern := trigger[1 : len(trigger)-1]
				if matched, _ := regexp.MatchString(pattern, input); matched {
					return skill
				}
			} else {
				// Plain text trigger - case insensitive contains
				if strings.Contains(input, strings.ToLower(trigger)) {
					return skill
				}
			}
		}
	}

	return nil
}

// Get returns a skill by name
func (l *Loader) Get(name string) (*Skill, bool) {
	skill, ok := l.skills[name]
	return skill, ok
}

// List returns all loaded skills
func (l *Loader) List() []*Skill {
	skills := make([]*Skill, 0, len(l.skills))
	for _, skill := range l.skills {
		skills = append(skills, skill)
	}
	return skills
}

// Reload reloads all skills from disk
func (l *Loader) Reload() {
	l.skills = make(map[string]*Skill)
	l.loadSkills()
}

// AddDir adds a skill directory to search
func (l *Loader) AddDir(dir string) {
	l.skillDirs = append(l.skillDirs, dir)
	l.loadFromDir(dir)
}

// GetPrompt returns the full prompt for a skill
func (s *Skill) GetPrompt() string {
	return s.Content
}

// Matches checks if a skill matches the given input
func (s *Skill) Matches(input string) bool {
	input = strings.ToLower(input)

	for _, trigger := range s.Triggers {
		if strings.HasPrefix(trigger, "/") && strings.HasSuffix(trigger, "/") {
			pattern := trigger[1 : len(trigger)-1]
			if matched, _ := regexp.MatchString(pattern, input); matched {
				return true
			}
		} else {
			if strings.Contains(input, strings.ToLower(trigger)) {
				return true
			}
		}
	}

	return false
}
