package cmd

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
)

func TestCLICommands_FollowStructuralConventions(t *testing.T) {
	t.Parallel()

	_, currentFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve test file path")
	}
	commandRoot := filepath.Dir(currentFile)

	disallowedDepsNames := map[string]struct{}{
		"CommandDeps": {}, "GetCommandDeps": {}, "ListCommandDeps": {},
		"CreateCommandDeps": {}, "DeleteCommandDeps": {}, "StatusCommandDeps": {},
		"SubmitCommandDeps": {}, "StartCommandDeps": {}, "CancelCommandDeps": {},
		"InfoCommandDeps": {}, "ExpireCommandDeps": {}, "VerifyCommandDeps": {},
	}

	err := filepath.WalkDir(commandRoot, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}

		fileset := token.NewFileSet()
		parsed, err := parser.ParseFile(fileset, path, nil, 0)
		if err != nil {
			t.Errorf("parse %s: %v", path, err)
			return nil
		}

		requiredFlags := make(map[string]token.Pos)
		markedRequiredFlags := make(map[string]struct{})

		ast.Inspect(parsed, func(node ast.Node) bool {
			switch value := node.(type) {
			case *ast.CallExpr:
				collectRequiredFlagConvention(value, requiredFlags, markedRequiredFlags)
			case *ast.CompositeLit:
				selector, ok := value.Type.(*ast.SelectorExpr)
				if !ok || selector.Sel.Name != "Command" {
					return true
				}
				fields := keyedFields(value.Elts)
				if _, ok := fields["Short"]; !ok {
					t.Errorf("%s:%d: cobra command is missing Short", path, fileset.Position(value.Pos()).Line)
				}
				if _, ok := fields["Long"]; !ok {
					t.Errorf("%s:%d: cobra command is missing Long", path, fileset.Position(value.Pos()).Line)
				}
				if _, ok := fields["RunE"]; ok {
					t.Errorf("%s:%d: assign cmd.RunE after flag registration", path, fileset.Position(value.Pos()).Line)
				}
			case *ast.TypeSpec:
				if _, ok := value.Type.(*ast.InterfaceType); !ok {
					return true
				}
				if _, disallowed := disallowedDepsNames[value.Name.Name]; disallowed {
					t.Errorf("%s:%d: dependency interface %s is not path-descriptive", path, fileset.Position(value.Pos()).Line, value.Name.Name)
				}
			case *ast.FuncDecl:
				if !strings.HasPrefix(value.Name.Name, "run") || value.Type.Params == nil {
					return true
				}
				for _, field := range value.Type.Params.List {
					array, ok := field.Type.(*ast.ArrayType)
					if !ok || array.Len != nil || len(field.Names) != 1 || field.Names[0].Name != "_" {
						continue
					}
					if ident, ok := array.Elt.(*ast.Ident); ok && ident.Name == "string" {
						t.Errorf("%s:%d: run function has an unused positional-args parameter", path, fileset.Position(field.Pos()).Line)
					}
				}
			case *ast.AssignStmt:
				if len(value.Lhs) < 2 || !isBlankIdentifier(value.Lhs[1]) {
					return true
				}
				for _, rhs := range value.Rhs {
					call, ok := rhs.(*ast.CallExpr)
					if !ok || !isFlagGetter(call.Fun) {
						continue
					}
					t.Errorf("%s:%d: flag getter error is discarded", path, fileset.Position(value.Pos()).Line)
				}
			}
			return true
		})
		for name, position := range requiredFlags {
			if _, ok := markedRequiredFlags[name]; !ok {
				t.Errorf("%s:%d: flag %q is documented as required but not marked required", path, fileset.Position(position).Line, name)
			}
		}

		return nil
	})
	if err != nil {
		t.Fatalf("walk CLI commands: %v", err)
	}
}

func collectRequiredFlagConvention(call *ast.CallExpr, required map[string]token.Pos, marked map[string]struct{}) {
	selector, ok := call.Fun.(*ast.SelectorExpr)
	if !ok || len(call.Args) == 0 {
		return
	}

	if selector.Sel.Name == "MarkFlagRequired" {
		if name, ok := stringLiteral(call.Args[0]); ok {
			marked[name] = struct{}{}
		}
		return
	}

	if !strings.HasPrefix(selector.Sel.Name, "String") &&
		!strings.HasPrefix(selector.Sel.Name, "Bool") &&
		!strings.HasPrefix(selector.Sel.Name, "Int") &&
		!strings.HasPrefix(selector.Sel.Name, "Float") {
		return
	}
	name, nameOK := stringLiteral(call.Args[0])
	usage, usageOK := stringLiteral(call.Args[len(call.Args)-1])
	if nameOK && usageOK && strings.Contains(usage, "(required)") {
		required[name] = call.Pos()
	}
}

func stringLiteral(expr ast.Expr) (string, bool) {
	literal, ok := expr.(*ast.BasicLit)
	if !ok || literal.Kind != token.STRING {
		return "", false
	}
	value, err := strconv.Unquote(literal.Value)
	return value, err == nil
}

func keyedFields(elements []ast.Expr) map[string]ast.Expr {
	fields := make(map[string]ast.Expr, len(elements))
	for _, element := range elements {
		keyed, ok := element.(*ast.KeyValueExpr)
		if !ok {
			continue
		}
		key, ok := keyed.Key.(*ast.Ident)
		if ok {
			fields[key.Name] = keyed.Value
		}
	}
	return fields
}

func isBlankIdentifier(expr ast.Expr) bool {
	ident, ok := expr.(*ast.Ident)
	return ok && ident.Name == "_"
}

func isFlagGetter(expr ast.Expr) bool {
	selector, ok := expr.(*ast.SelectorExpr)
	if !ok || !strings.HasPrefix(selector.Sel.Name, "Get") {
		return false
	}
	flagsCall, ok := selector.X.(*ast.CallExpr)
	if !ok {
		return false
	}
	flagsSelector, ok := flagsCall.Fun.(*ast.SelectorExpr)
	return ok && (flagsSelector.Sel.Name == "Flags" || flagsSelector.Sel.Name == "PersistentFlags")
}
