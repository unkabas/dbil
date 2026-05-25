// Command lint-auth statically verifies that every HTTP route mounted in
// internal/server/handlers/handlers.go is either inside a chi.Group whose
// body uses auth.RequireAuth(...) or in the explicit unauthed allowlist.
//
// This enforces, mechanically, the spec's "hybrid solo/team auth risk"
// mitigation: a single auth code path, with no opt-out branches scattered
// across the handler tree.
//
// Run from the repo root:
//
//	go run ./scripts/lint-auth
//
// Exit codes: 0 = clean, 1 = violations found, 2 = parse error.
package main

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
)

// unauthedRoutes are the only top-level routes allowed to be mounted
// outside auth.RequireAuth. Add new entries deliberately, with reviewer
// awareness that this is the auth bypass list.
var unauthedRoutes = map[string]bool{
	"/healthz":          true,
	"/api/auth/login":   true,
	"/api/openapi.yaml": true,
	"/api/docs":         true,
}

const handlersFile = "internal/server/handlers/handlers.go"

func main() {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, handlersFile, nil, parser.ParseComments)
	if err != nil {
		fmt.Fprintln(os.Stderr, "lint-auth: parse:", err)
		os.Exit(2)
	}

	mount := findFunc(f, "Mount")
	if mount == nil {
		fmt.Fprintln(os.Stderr, "lint-auth: Mount function not found in", handlersFile)
		os.Exit(2)
	}

	var violations []string
	for _, stmt := range mount.Body.List {
		exprStmt, ok := stmt.(*ast.ExprStmt)
		if !ok {
			continue
		}
		call, ok := exprStmt.X.(*ast.CallExpr)
		if !ok {
			continue
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			continue
		}
		switch sel.Sel.Name {
		case "Get", "Post", "Put", "Patch", "Delete":
			if len(call.Args) == 0 {
				continue
			}
			path := stringLit(call.Args[0])
			if path == "" {
				violations = append(violations, fmt.Sprintf(
					"top-level route at %s: non-literal path; lint-auth cannot verify it is allow-listed",
					fset.Position(call.Pos()),
				))
				continue
			}
			if !unauthedRoutes[path] {
				violations = append(violations, fmt.Sprintf(
					"%s: top-level route %q is not in the unauthed allowlist — mount it under auth.RequireAuth",
					fset.Position(call.Pos()), path,
				))
			}
		case "Group":
			if !groupHasRequireAuth(call) {
				for _, route := range groupRoutes(call) {
					violations = append(violations, fmt.Sprintf(
						"%s: route %q is inside a chi.Group that does not Use(auth.RequireAuth(...))",
						fset.Position(call.Pos()), route,
					))
				}
			}
		}
	}

	if len(violations) > 0 {
		for _, v := range violations {
			fmt.Fprintln(os.Stderr, "lint-auth violation:", v)
		}
		os.Exit(1)
	}
	fmt.Println("lint-auth: ok — every handler is correctly gated")
}

func findFunc(f *ast.File, name string) *ast.FuncDecl {
	for _, decl := range f.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok {
			continue
		}
		if fn.Name.Name == name {
			return fn
		}
	}
	return nil
}

func stringLit(e ast.Expr) string {
	lit, ok := e.(*ast.BasicLit)
	if !ok || lit.Kind != token.STRING {
		return ""
	}
	return lit.Value[1 : len(lit.Value)-1]
}

func groupHasRequireAuth(call *ast.CallExpr) bool {
	if len(call.Args) == 0 {
		return false
	}
	fnLit, ok := call.Args[0].(*ast.FuncLit)
	if !ok {
		return false
	}
	found := false
	ast.Inspect(fnLit.Body, func(n ast.Node) bool {
		c, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		sel, ok := c.Fun.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		if sel.Sel.Name != "Use" {
			return true
		}
		for _, arg := range c.Args {
			ac, ok := arg.(*ast.CallExpr)
			if !ok {
				continue
			}
			asel, ok := ac.Fun.(*ast.SelectorExpr)
			if !ok {
				continue
			}
			if asel.Sel.Name == "RequireAuth" {
				found = true
				return false
			}
		}
		return true
	})
	return found
}

func groupRoutes(call *ast.CallExpr) []string {
	if len(call.Args) == 0 {
		return nil
	}
	fnLit, ok := call.Args[0].(*ast.FuncLit)
	if !ok {
		return nil
	}
	var routes []string
	ast.Inspect(fnLit.Body, func(n ast.Node) bool {
		c, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		sel, ok := c.Fun.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		switch sel.Sel.Name {
		case "Get", "Post", "Put", "Patch", "Delete":
			if len(c.Args) > 0 {
				if p := stringLit(c.Args[0]); p != "" {
					routes = append(routes, p)
				}
			}
		}
		return true
	})
	return routes
}
