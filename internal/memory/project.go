package memory

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"path/filepath"
	"strings"
	"time"
)

// ProjectMemory stores project-specific patterns and knowledge
type ProjectMemory struct {
	store       *Store
	projectPath string
}

// Pattern represents a code pattern observed in the project
type Pattern struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Examples    []string `json:"examples"` // File paths with examples
	Tags        []string `json:"tags"`
}

// Convention represents a coding convention in the project
type Convention struct {
	Category    string `json:"category"` // naming, formatting, structure, etc.
	Description string `json:"description"`
	Example     string `json:"example"`
}

// Architecture represents an architectural decision
type Architecture struct {
	Component   string   `json:"component"`
	Description string   `json:"description"`
	Files       []string `json:"files"`   // Key files for this component
	Depends     []string `json:"depends"` // Other components this depends on
}

// NewProjectMemory creates a new project memory
func NewProjectMemory(projectPath string) (*ProjectMemory, error) {
	// Create store in project's .vecai directory
	storePath := filepath.Join(projectPath, ".vecai", "memory")
	store, err := NewStore(storePath)
	if err != nil {
		return nil, err
	}

	return &ProjectMemory{
		store:       store,
		projectPath: projectPath,
	}, nil
}

// AddPattern stores a code pattern
func (p *ProjectMemory) AddPattern(pattern Pattern) error {
	id := p.generateID("pattern", pattern.Name)

	entry := &MemoryEntry{
		ID:      id,
		Type:    MemoryTypeProject,
		Content: p.serializePattern(pattern),
		Metadata: map[string]string{
			"subtype": "pattern",
			"name":    pattern.Name,
		},
	}

	return p.store.Add(entry)
}

// GetPatterns retrieves all stored patterns
func (p *ProjectMemory) GetPatterns() []Pattern {
	entries := p.store.List(MemoryTypeProject)

	var patterns []Pattern
	for _, entry := range entries {
		if entry.Metadata["subtype"] == "pattern" {
			if pattern := p.parsePattern(entry.Content); pattern != nil {
				patterns = append(patterns, *pattern)
			}
		}
	}
	return patterns
}

// FindPatterns searches for patterns matching tags
func (p *ProjectMemory) FindPatterns(tags []string) []Pattern {
	patterns := p.GetPatterns()

	var matches []Pattern
	for _, pattern := range patterns {
		for _, tag := range tags {
			for _, ptag := range pattern.Tags {
				if strings.EqualFold(tag, ptag) {
					matches = append(matches, pattern)
					break
				}
			}
		}
	}
	return matches
}

// AddConvention stores a coding convention
func (p *ProjectMemory) AddConvention(conv Convention) error {
	id := p.generateID("convention", conv.Category+"-"+conv.Description[:min(20, len(conv.Description))])

	entry := &MemoryEntry{
		ID:      id,
		Type:    MemoryTypeProject,
		Content: p.serializeConvention(conv),
		Metadata: map[string]string{
			"subtype":  "convention",
			"category": conv.Category,
		},
	}

	return p.store.Add(entry)
}

// GetConventions retrieves all conventions
func (p *ProjectMemory) GetConventions() []Convention {
	entries := p.store.List(MemoryTypeProject)

	var conventions []Convention
	for _, entry := range entries {
		if entry.Metadata["subtype"] == "convention" {
			if conv := p.parseConvention(entry.Content); conv != nil {
				conventions = append(conventions, *conv)
			}
		}
	}
	return conventions
}

// GetConventionsByCategory retrieves conventions for a category
func (p *ProjectMemory) GetConventionsByCategory(category string) []Convention {
	conventions := p.GetConventions()

	var matches []Convention
	for _, conv := range conventions {
		if strings.EqualFold(conv.Category, category) {
			matches = append(matches, conv)
		}
	}
	return matches
}

// AddArchitecture stores architectural information
func (p *ProjectMemory) AddArchitecture(arch Architecture) error {
	id := p.generateID("architecture", arch.Component)

	entry := &MemoryEntry{
		ID:      id,
		Type:    MemoryTypeProject,
		Content: p.serializeArchitecture(arch),
		Metadata: map[string]string{
			"subtype":   "architecture",
			"component": arch.Component,
		},
	}

	return p.store.Add(entry)
}

// GetArchitecture retrieves all architectural information
func (p *ProjectMemory) GetArchitecture() []Architecture {
	entries := p.store.List(MemoryTypeProject)

	var archs []Architecture
	for _, entry := range entries {
		if entry.Metadata["subtype"] == "architecture" {
			if arch := p.parseArchitecture(entry.Content); arch != nil {
				archs = append(archs, *arch)
			}
		}
	}
	return archs
}

// GetComponent retrieves architecture for a specific component
func (p *ProjectMemory) GetComponent(name string) *Architecture {
	archs := p.GetArchitecture()

	for _, arch := range archs {
		if strings.EqualFold(arch.Component, name) {
			return &arch
		}
	}
	return nil
}

// GetProjectSummary returns a summary of project knowledge
func (p *ProjectMemory) GetProjectSummary() string {
	var sb strings.Builder

	patterns := p.GetPatterns()
	if len(patterns) > 0 {
		sb.WriteString(fmt.Sprintf("## Patterns (%d)\n", len(patterns)))
		for _, pattern := range patterns[:min(5, len(patterns))] {
			sb.WriteString(fmt.Sprintf("- %s: %s\n", pattern.Name, pattern.Description))
		}
		sb.WriteString("\n")
	}

	conventions := p.GetConventions()
	if len(conventions) > 0 {
		sb.WriteString(fmt.Sprintf("## Conventions (%d)\n", len(conventions)))
		for _, conv := range conventions[:min(5, len(conventions))] {
			sb.WriteString(fmt.Sprintf("- [%s] %s\n", conv.Category, conv.Description))
		}
		sb.WriteString("\n")
	}

	archs := p.GetArchitecture()
	if len(archs) > 0 {
		sb.WriteString(fmt.Sprintf("## Architecture (%d components)\n", len(archs)))
		for _, arch := range archs[:min(5, len(archs))] {
			sb.WriteString(fmt.Sprintf("- %s: %s\n", arch.Component, arch.Description))
		}
	}

	return sb.String()
}

// Close closes the project memory store
func (p *ProjectMemory) Close() error {
	return p.store.Close()
}

// Helper methods

func (p *ProjectMemory) generateID(prefix, name string) string {
	hash := sha256.Sum256([]byte(name + time.Now().String()))
	return prefix + "-" + hex.EncodeToString(hash[:8])
}

func (p *ProjectMemory) serializePattern(pattern Pattern) string {
	return fmt.Sprintf("NAME:%s\nDESC:%s\nEXAMPLES:%s\nTAGS:%s",
		pattern.Name,
		pattern.Description,
		strings.Join(pattern.Examples, ","),
		strings.Join(pattern.Tags, ","))
}

func (p *ProjectMemory) parsePattern(content string) *Pattern {
	pattern := &Pattern{}
	for _, line := range strings.Split(content, "\n") {
		if strings.HasPrefix(line, "NAME:") {
			pattern.Name = strings.TrimPrefix(line, "NAME:")
		} else if strings.HasPrefix(line, "DESC:") {
			pattern.Description = strings.TrimPrefix(line, "DESC:")
		} else if strings.HasPrefix(line, "EXAMPLES:") {
			examples := strings.TrimPrefix(line, "EXAMPLES:")
			if examples != "" {
				pattern.Examples = strings.Split(examples, ",")
			}
		} else if strings.HasPrefix(line, "TAGS:") {
			tags := strings.TrimPrefix(line, "TAGS:")
			if tags != "" {
				pattern.Tags = strings.Split(tags, ",")
			}
		}
	}
	if pattern.Name == "" {
		return nil
	}
	return pattern
}

func (p *ProjectMemory) serializeConvention(conv Convention) string {
	return fmt.Sprintf("CATEGORY:%s\nDESC:%s\nEXAMPLE:%s",
		conv.Category,
		conv.Description,
		conv.Example)
}

func (p *ProjectMemory) parseConvention(content string) *Convention {
	conv := &Convention{}
	for _, line := range strings.Split(content, "\n") {
		if strings.HasPrefix(line, "CATEGORY:") {
			conv.Category = strings.TrimPrefix(line, "CATEGORY:")
		} else if strings.HasPrefix(line, "DESC:") {
			conv.Description = strings.TrimPrefix(line, "DESC:")
		} else if strings.HasPrefix(line, "EXAMPLE:") {
			conv.Example = strings.TrimPrefix(line, "EXAMPLE:")
		}
	}
	if conv.Description == "" {
		return nil
	}
	return conv
}

func (p *ProjectMemory) serializeArchitecture(arch Architecture) string {
	return fmt.Sprintf("COMPONENT:%s\nDESC:%s\nFILES:%s\nDEPENDS:%s",
		arch.Component,
		arch.Description,
		strings.Join(arch.Files, ","),
		strings.Join(arch.Depends, ","))
}

func (p *ProjectMemory) parseArchitecture(content string) *Architecture {
	arch := &Architecture{}
	for _, line := range strings.Split(content, "\n") {
		if strings.HasPrefix(line, "COMPONENT:") {
			arch.Component = strings.TrimPrefix(line, "COMPONENT:")
		} else if strings.HasPrefix(line, "DESC:") {
			arch.Description = strings.TrimPrefix(line, "DESC:")
		} else if strings.HasPrefix(line, "FILES:") {
			files := strings.TrimPrefix(line, "FILES:")
			if files != "" {
				arch.Files = strings.Split(files, ",")
			}
		} else if strings.HasPrefix(line, "DEPENDS:") {
			depends := strings.TrimPrefix(line, "DEPENDS:")
			if depends != "" {
				arch.Depends = strings.Split(depends, ",")
			}
		}
	}
	if arch.Component == "" {
		return nil
	}
	return arch
}
