package tools

import (
	"context"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"strings"
)

// ASTTool parses Go files and extracts structured code information
type ASTTool struct{}

func (t *ASTTool) Name() string {
	return "ast_parse"
}

func (t *ASTTool) Description() string {
	return "Parse Go source files and extract structured information: functions, types, imports, and constants with line numbers. Useful for understanding code structure without reading entire files."
}

func (t *ASTTool) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"path": map[string]any{
				"type":        "string",
				"description": "Path to the Go file to parse.",
			},
			"include": map[string]any{
				"type":        "array",
				"items":       map[string]any{"type": "string"},
				"description": "What to include: 'functions', 'types', 'imports', 'constants', 'variables'. Default: all.",
			},
		},
		"required": []string{"path"},
	}
}

func (t *ASTTool) Permission() PermissionLevel {
	return PermissionRead
}

// ParseResult holds the parsed AST information
type ParseResult struct {
	Functions []FunctionInfo `json:"functions,omitempty"`
	Types     []TypeInfo     `json:"types,omitempty"`
	Imports   []ImportInfo   `json:"imports,omitempty"`
	Constants []ConstInfo    `json:"constants,omitempty"`
	Variables []VarInfo      `json:"variables,omitempty"`
}

// FunctionInfo holds function metadata
type FunctionInfo struct {
	Name       string `json:"name"`
	Receiver   string `json:"receiver,omitempty"`
	Signature  string `json:"signature"`
	Line       int    `json:"line"`
	EndLine    int    `json:"end_line"`
	Doc        string `json:"doc,omitempty"`
	Exported   bool   `json:"exported"`
}

// TypeInfo holds type metadata
type TypeInfo struct {
	Name     string   `json:"name"`
	Kind     string   `json:"kind"` // struct, interface, alias
	Line     int      `json:"line"`
	Fields   []string `json:"fields,omitempty"`
	Methods  []string `json:"methods,omitempty"`
	Doc      string   `json:"doc,omitempty"`
	Exported bool     `json:"exported"`
}

// ImportInfo holds import metadata
type ImportInfo struct {
	Path  string `json:"path"`
	Alias string `json:"alias,omitempty"`
	Line  int    `json:"line"`
}

// ConstInfo holds constant metadata
type ConstInfo struct {
	Name     string `json:"name"`
	Type     string `json:"type,omitempty"`
	Value    string `json:"value,omitempty"`
	Line     int    `json:"line"`
	Exported bool   `json:"exported"`
}

// VarInfo holds variable metadata
type VarInfo struct {
	Name     string `json:"name"`
	Type     string `json:"type,omitempty"`
	Line     int    `json:"line"`
	Exported bool   `json:"exported"`
}

func (t *ASTTool) Execute(ctx context.Context, input map[string]any) (string, error) {
	path, ok := input["path"].(string)
	if !ok || path == "" {
		return "", fmt.Errorf("path is required")
	}

	// Resolve path
	absPath, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("invalid path: %w", err)
	}

	// Validate path is within project directory
	if err := ValidatePath(absPath); err != nil {
		return "", err
	}

	// Check if it's a Go file
	if !strings.HasSuffix(absPath, ".go") {
		return "", fmt.Errorf("ast_parse only works with Go files (*.go)")
	}

	// Determine what to include
	includeAll := true
	includeSet := make(map[string]bool)
	if includes, ok := input["include"].([]any); ok && len(includes) > 0 {
		includeAll = false
		for _, inc := range includes {
			if s, ok := inc.(string); ok {
				includeSet[s] = true
			}
		}
	}

	// Parse the file
	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, absPath, nil, parser.ParseComments)
	if err != nil {
		return "", fmt.Errorf("failed to parse Go file: %w", err)
	}

	result := &ParseResult{}

	// Extract imports
	if includeAll || includeSet["imports"] {
		for _, imp := range node.Imports {
			info := ImportInfo{
				Path: strings.Trim(imp.Path.Value, `"`),
				Line: fset.Position(imp.Pos()).Line,
			}
			if imp.Name != nil {
				info.Alias = imp.Name.Name
			}
			result.Imports = append(result.Imports, info)
		}
	}

	// Extract declarations
	for _, decl := range node.Decls {
		switch d := decl.(type) {
		case *ast.FuncDecl:
			if includeAll || includeSet["functions"] {
				result.Functions = append(result.Functions, t.parseFuncDecl(d, fset))
			}

		case *ast.GenDecl:
			switch d.Tok {
			case token.TYPE:
				if includeAll || includeSet["types"] {
					for _, spec := range d.Specs {
						if ts, ok := spec.(*ast.TypeSpec); ok {
							result.Types = append(result.Types, t.parseTypeSpec(ts, d, fset))
						}
					}
				}

			case token.CONST:
				if includeAll || includeSet["constants"] {
					for _, spec := range d.Specs {
						if vs, ok := spec.(*ast.ValueSpec); ok {
							for i, name := range vs.Names {
								info := ConstInfo{
									Name:     name.Name,
									Line:     fset.Position(name.Pos()).Line,
									Exported: ast.IsExported(name.Name),
								}
								if vs.Type != nil {
									info.Type = t.typeString(vs.Type)
								}
								if i < len(vs.Values) {
									info.Value = t.exprString(vs.Values[i])
								}
								result.Constants = append(result.Constants, info)
							}
						}
					}
				}

			case token.VAR:
				if includeAll || includeSet["variables"] {
					for _, spec := range d.Specs {
						if vs, ok := spec.(*ast.ValueSpec); ok {
							for _, name := range vs.Names {
								info := VarInfo{
									Name:     name.Name,
									Line:     fset.Position(name.Pos()).Line,
									Exported: ast.IsExported(name.Name),
								}
								if vs.Type != nil {
									info.Type = t.typeString(vs.Type)
								}
								result.Variables = append(result.Variables, info)
							}
						}
					}
				}
			}
		}
	}

	// Format output
	return t.formatResult(result, filepath.Base(absPath)), nil
}

func (t *ASTTool) parseFuncDecl(fn *ast.FuncDecl, fset *token.FileSet) FunctionInfo {
	info := FunctionInfo{
		Name:     fn.Name.Name,
		Line:     fset.Position(fn.Pos()).Line,
		EndLine:  fset.Position(fn.End()).Line,
		Exported: ast.IsExported(fn.Name.Name),
	}

	// Get receiver
	if fn.Recv != nil && len(fn.Recv.List) > 0 {
		info.Receiver = t.typeString(fn.Recv.List[0].Type)
	}

	// Build signature
	info.Signature = t.buildFuncSignature(fn)

	// Get doc
	if fn.Doc != nil {
		info.Doc = strings.TrimSpace(fn.Doc.Text())
	}

	return info
}

func (t *ASTTool) parseTypeSpec(ts *ast.TypeSpec, gd *ast.GenDecl, fset *token.FileSet) TypeInfo {
	info := TypeInfo{
		Name:     ts.Name.Name,
		Line:     fset.Position(ts.Pos()).Line,
		Exported: ast.IsExported(ts.Name.Name),
	}

	// Get doc
	if gd.Doc != nil {
		info.Doc = strings.TrimSpace(gd.Doc.Text())
	} else if ts.Doc != nil {
		info.Doc = strings.TrimSpace(ts.Doc.Text())
	}

	// Determine kind and extract fields/methods
	switch typ := ts.Type.(type) {
	case *ast.StructType:
		info.Kind = "struct"
		if typ.Fields != nil {
			for _, field := range typ.Fields.List {
				fieldType := t.typeString(field.Type)
				for _, name := range field.Names {
					info.Fields = append(info.Fields, fmt.Sprintf("%s %s", name.Name, fieldType))
				}
				// Embedded field
				if len(field.Names) == 0 {
					info.Fields = append(info.Fields, fieldType)
				}
			}
		}

	case *ast.InterfaceType:
		info.Kind = "interface"
		if typ.Methods != nil {
			for _, method := range typ.Methods.List {
				for _, name := range method.Names {
					info.Methods = append(info.Methods, name.Name)
				}
				// Embedded interface
				if len(method.Names) == 0 {
					info.Methods = append(info.Methods, t.typeString(method.Type))
				}
			}
		}

	default:
		info.Kind = "alias"
	}

	return info
}

func (t *ASTTool) buildFuncSignature(fn *ast.FuncDecl) string {
	var sb strings.Builder

	sb.WriteString("func ")

	// Receiver
	if fn.Recv != nil && len(fn.Recv.List) > 0 {
		sb.WriteString("(")
		if len(fn.Recv.List[0].Names) > 0 {
			sb.WriteString(fn.Recv.List[0].Names[0].Name)
			sb.WriteString(" ")
		}
		sb.WriteString(t.typeString(fn.Recv.List[0].Type))
		sb.WriteString(") ")
	}

	sb.WriteString(fn.Name.Name)
	sb.WriteString("(")

	// Parameters
	if fn.Type.Params != nil {
		params := []string{}
		for _, p := range fn.Type.Params.List {
			pType := t.typeString(p.Type)
			if len(p.Names) > 0 {
				names := []string{}
				for _, n := range p.Names {
					names = append(names, n.Name)
				}
				params = append(params, strings.Join(names, ", ")+" "+pType)
			} else {
				params = append(params, pType)
			}
		}
		sb.WriteString(strings.Join(params, ", "))
	}
	sb.WriteString(")")

	// Results
	if fn.Type.Results != nil && len(fn.Type.Results.List) > 0 {
		sb.WriteString(" ")
		if len(fn.Type.Results.List) == 1 && len(fn.Type.Results.List[0].Names) == 0 {
			sb.WriteString(t.typeString(fn.Type.Results.List[0].Type))
		} else {
			sb.WriteString("(")
			results := []string{}
			for _, r := range fn.Type.Results.List {
				rType := t.typeString(r.Type)
				if len(r.Names) > 0 {
					names := []string{}
					for _, n := range r.Names {
						names = append(names, n.Name)
					}
					results = append(results, strings.Join(names, ", ")+" "+rType)
				} else {
					results = append(results, rType)
				}
			}
			sb.WriteString(strings.Join(results, ", "))
			sb.WriteString(")")
		}
	}

	return sb.String()
}

func (t *ASTTool) typeString(expr ast.Expr) string {
	switch e := expr.(type) {
	case *ast.Ident:
		return e.Name
	case *ast.StarExpr:
		return "*" + t.typeString(e.X)
	case *ast.SelectorExpr:
		return t.typeString(e.X) + "." + e.Sel.Name
	case *ast.ArrayType:
		if e.Len == nil {
			return "[]" + t.typeString(e.Elt)
		}
		return fmt.Sprintf("[%s]%s", t.exprString(e.Len), t.typeString(e.Elt))
	case *ast.MapType:
		return fmt.Sprintf("map[%s]%s", t.typeString(e.Key), t.typeString(e.Value))
	case *ast.ChanType:
		switch e.Dir {
		case ast.SEND:
			return "chan<- " + t.typeString(e.Value)
		case ast.RECV:
			return "<-chan " + t.typeString(e.Value)
		default:
			return "chan " + t.typeString(e.Value)
		}
	case *ast.FuncType:
		return "func(...)"
	case *ast.InterfaceType:
		return "interface{}"
	case *ast.StructType:
		return "struct{...}"
	case *ast.Ellipsis:
		return "..." + t.typeString(e.Elt)
	default:
		return "unknown"
	}
}

func (t *ASTTool) exprString(expr ast.Expr) string {
	switch e := expr.(type) {
	case *ast.BasicLit:
		return e.Value
	case *ast.Ident:
		return e.Name
	case *ast.SelectorExpr:
		return t.exprString(e.X) + "." + e.Sel.Name
	case *ast.CallExpr:
		return t.exprString(e.Fun) + "(...)"
	case *ast.CompositeLit:
		return t.typeString(e.Type) + "{...}"
	default:
		return "..."
	}
}

func (t *ASTTool) formatResult(result *ParseResult, filename string) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("=== AST Parse: %s ===\n\n", filename))

	// Imports
	if len(result.Imports) > 0 {
		sb.WriteString("## Imports\n")
		for _, imp := range result.Imports {
			if imp.Alias != "" && imp.Alias != "_" && imp.Alias != "." {
				sb.WriteString(fmt.Sprintf("  L%d: %s %q\n", imp.Line, imp.Alias, imp.Path))
			} else if imp.Alias == "_" {
				sb.WriteString(fmt.Sprintf("  L%d: _ %q (side-effect)\n", imp.Line, imp.Path))
			} else {
				sb.WriteString(fmt.Sprintf("  L%d: %q\n", imp.Line, imp.Path))
			}
		}
		sb.WriteString("\n")
	}

	// Types
	if len(result.Types) > 0 {
		sb.WriteString("## Types\n")
		for _, typ := range result.Types {
			exported := ""
			if typ.Exported {
				exported = " (exported)"
			}
			sb.WriteString(fmt.Sprintf("  L%d: type %s %s%s\n", typ.Line, typ.Name, typ.Kind, exported))
			if typ.Doc != "" {
				sb.WriteString(fmt.Sprintf("       Doc: %s\n", truncate(typ.Doc, 60)))
			}
			if len(typ.Fields) > 0 {
				sb.WriteString(fmt.Sprintf("       Fields: %s\n", strings.Join(typ.Fields, ", ")))
			}
			if len(typ.Methods) > 0 {
				sb.WriteString(fmt.Sprintf("       Methods: %s\n", strings.Join(typ.Methods, ", ")))
			}
		}
		sb.WriteString("\n")
	}

	// Constants
	if len(result.Constants) > 0 {
		sb.WriteString("## Constants\n")
		for _, c := range result.Constants {
			val := ""
			if c.Value != "" {
				val = " = " + truncate(c.Value, 30)
			}
			sb.WriteString(fmt.Sprintf("  L%d: %s%s\n", c.Line, c.Name, val))
		}
		sb.WriteString("\n")
	}

	// Variables
	if len(result.Variables) > 0 {
		sb.WriteString("## Variables\n")
		for _, v := range result.Variables {
			typ := ""
			if v.Type != "" {
				typ = " " + v.Type
			}
			sb.WriteString(fmt.Sprintf("  L%d: %s%s\n", v.Line, v.Name, typ))
		}
		sb.WriteString("\n")
	}

	// Functions
	if len(result.Functions) > 0 {
		sb.WriteString("## Functions\n")
		for _, fn := range result.Functions {
			sb.WriteString(fmt.Sprintf("  L%d-%d: %s\n", fn.Line, fn.EndLine, fn.Signature))
			if fn.Doc != "" {
				sb.WriteString(fmt.Sprintf("          Doc: %s\n", truncate(fn.Doc, 60)))
			}
		}
		sb.WriteString("\n")
	}

	return sb.String()
}

func truncate(s string, maxLen int) string {
	// Replace newlines with spaces for single-line display
	s = strings.ReplaceAll(s, "\n", " ")
	if len(s) > maxLen {
		return s[:maxLen-3] + "..."
	}
	return s
}
