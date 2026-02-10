package agent

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

const (
	repoMapMaxChars = 8000
	repoMapCacheTTL = 5 * time.Minute
)

// RepoMap generates a compact overview of the Go codebase's exported API.
type RepoMap struct {
	mu        sync.Mutex
	cached    string
	cachedAt  time.Time
	rootDir   string
}

// NewRepoMap creates a new RepoMap rooted at the given directory.
func NewRepoMap(rootDir string) *RepoMap {
	return &RepoMap{rootDir: rootDir}
}

// Get returns the cached repo map, rebuilding if stale.
func (rm *RepoMap) Get() string {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	if rm.cached != "" && time.Since(rm.cachedAt) < repoMapCacheTTL {
		return rm.cached
	}

	rm.cached = rm.build()
	rm.cachedAt = time.Now()
	return rm.cached
}

// build walks .go files (skipping tests, vendor, .git) and extracts exported symbols.
func (rm *RepoMap) build() string {
	dirMap := make(map[string][]string)
	fset := token.NewFileSet()

	_ = filepath.Walk(rm.rootDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}

		// Skip directories
		name := info.Name()
		if info.IsDir() {
			if name == "vendor" || name == ".git" || name == ".vecai" || name == ".vecgrep" || name == "node_modules" {
				return filepath.SkipDir
			}
			return nil
		}

		// Only .go files, skip tests
		if !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			return nil
		}

		f, parseErr := parser.ParseFile(fset, path, nil, parser.SkipObjectResolution)
		if parseErr != nil {
			return nil
		}

		relDir, _ := filepath.Rel(rm.rootDir, filepath.Dir(path))
		if relDir == "" {
			relDir = "."
		}

		for _, decl := range f.Decls {
			switch d := decl.(type) {
			case *ast.FuncDecl:
				if !d.Name.IsExported() {
					continue
				}
				sig := formatFuncSig(d)
				dirMap[relDir] = append(dirMap[relDir], sig)

			case *ast.GenDecl:
				for _, spec := range d.Specs {
					switch s := spec.(type) {
					case *ast.TypeSpec:
						if !s.Name.IsExported() {
							continue
						}
						kind := "type"
						switch s.Type.(type) {
						case *ast.StructType:
							kind = "struct"
						case *ast.InterfaceType:
							kind = "interface"
						}
						dirMap[relDir] = append(dirMap[relDir], fmt.Sprintf("type %s %s", s.Name.Name, kind))
					}
				}
			}
		}

		return nil
	})

	// Sort directories for stable output
	var dirs []string
	for dir := range dirMap {
		dirs = append(dirs, dir)
	}
	sort.Strings(dirs)

	var sb strings.Builder
	sb.WriteString("## Repository Map\n\n")

	totalLen := sb.Len()
	truncated := false

	for _, dir := range dirs {
		symbols := dirMap[dir]
		// Deduplicate
		sort.Strings(symbols)
		symbols = dedup(symbols)

		section := fmt.Sprintf("### %s\n", dir)
		for _, sym := range symbols {
			section += fmt.Sprintf("  %s\n", sym)
		}
		section += "\n"

		if totalLen+len(section) > repoMapMaxChars {
			truncated = true
			break
		}
		sb.WriteString(section)
		totalLen += len(section)
	}

	if truncated {
		sb.WriteString("[... truncated]\n")
	}

	return sb.String()
}

// formatFuncSig formats a function declaration as a compact signature.
func formatFuncSig(d *ast.FuncDecl) string {
	var sb strings.Builder
	sb.WriteString("func ")
	if d.Recv != nil && len(d.Recv.List) > 0 {
		recv := d.Recv.List[0]
		sb.WriteString("(")
		sb.WriteString(typeString(recv.Type))
		sb.WriteString(").")
	}
	sb.WriteString(d.Name.Name)
	sb.WriteString("(")

	// Parameters
	if d.Type.Params != nil {
		var params []string
		for _, p := range d.Type.Params.List {
			typeName := typeString(p.Type)
			if len(p.Names) == 0 {
				params = append(params, typeName)
			} else {
				for range p.Names {
					params = append(params, typeName)
				}
			}
		}
		sb.WriteString(strings.Join(params, ", "))
	}
	sb.WriteString(")")

	// Return types
	if d.Type.Results != nil && len(d.Type.Results.List) > 0 {
		var rets []string
		for _, r := range d.Type.Results.List {
			rets = append(rets, typeString(r.Type))
		}
		if len(rets) == 1 {
			sb.WriteString(" ")
			sb.WriteString(rets[0])
		} else {
			sb.WriteString(" (")
			sb.WriteString(strings.Join(rets, ", "))
			sb.WriteString(")")
		}
	}

	return sb.String()
}

// typeString returns a compact string for an AST type expression.
func typeString(expr ast.Expr) string {
	switch t := expr.(type) {
	case *ast.Ident:
		return t.Name
	case *ast.StarExpr:
		return "*" + typeString(t.X)
	case *ast.SelectorExpr:
		return typeString(t.X) + "." + t.Sel.Name
	case *ast.ArrayType:
		return "[]" + typeString(t.Elt)
	case *ast.MapType:
		return "map[" + typeString(t.Key) + "]" + typeString(t.Value)
	case *ast.InterfaceType:
		return "interface{}"
	case *ast.FuncType:
		return "func(...)"
	case *ast.ChanType:
		return "chan " + typeString(t.Value)
	case *ast.Ellipsis:
		return "..." + typeString(t.Elt)
	default:
		return "any"
	}
}

// dedup removes consecutive duplicates from a sorted slice.
func dedup(s []string) []string {
	if len(s) <= 1 {
		return s
	}
	result := s[:1]
	for _, v := range s[1:] {
		if v != result[len(result)-1] {
			result = append(result, v)
		}
	}
	return result
}
