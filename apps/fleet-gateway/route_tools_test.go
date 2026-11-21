package main

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"testing"
)

type upstreamToolLoader func() (map[string]bool, error)

var prodUpstreamToolLoaders = map[string]upstreamToolLoader{
	"mcp-onboarding": func() (map[string]bool, error) {
		return onboardingToolNames(filepath.Join("..", "mcp-onboarding", "mcp.go"))
	},
	"mcp-kodeverk": func() (map[string]bool, error) {
		return mapSliceToolNames(filepath.Join("..", "mcp-kodeverk", "tools.go"), "baseToolDefinitions")
	},
	"mcp-dolly": func() (map[string]bool, error) {
		return dollyToolNames(filepath.Join("..", "mcp-dolly", "tools.go"))
	},
}

var devUpstreamToolLoaders = map[string]upstreamToolLoader{
	"fleet-echo": func() (map[string]bool, error) {
		return mapSliceToolNames(filepath.Join("echo", "main.go"), "serveModernEcho")
	},
	"mcp-onboarding": prodUpstreamToolLoaders["mcp-onboarding"],
	"mcp-kodeverk":   prodUpstreamToolLoaders["mcp-kodeverk"],
	"mcp-dolly":      prodUpstreamToolLoaders["mcp-dolly"],
	"mcp-team-catalog": func() (map[string]bool, error) {
		return mapSliceToolNames(filepath.Join("..", "mcp-team-catalog", "tools.go"), "baseToolDefinitions")
	},
}

func TestProdRouteToolsExistInUpstreams(t *testing.T) {
	checkRouteFileTools(t, "pilot-routes.prod.yaml", prodUpstreamToolLoaders)
}

func TestDevRouteToolsExistInUpstreams(t *testing.T) {
	checkRouteFileTools(t, "pilot-routes.dev.yaml", devUpstreamToolLoaders)
}

func TestUpstreamToolParsers(t *testing.T) {
	wantSamples := map[string][]string{
		"mcp-onboarding":   {"hello_world", "team_readiness"},
		"mcp-kodeverk":     {"list_kodeverk", "search_kodeverk"},
		"mcp-dolly":        {"dolly_current_user", "create_order"},
		"mcp-team-catalog": {"list_teams", "get_team", "get_member_memberships"},
		"fleet-echo":       {"whoami"},
	}
	for app, samples := range wantSamples {
		loader, ok := devUpstreamToolLoaders[app]
		if !ok {
			t.Fatalf("missing loader for %s", app)
		}
		got, err := loader()
		if err != nil {
			t.Fatalf("%s: %v", app, err)
		}
		for _, sample := range samples {
			if !got[sample] {
				t.Errorf("%s: expected parsed tools to contain %q, got %v", app, sample, got)
			}
		}
	}
}

// Control: unknown route tools are flagged while omitted upstream tools are allowed.
func TestDiffRouteTools(t *testing.T) {
	offered := map[string]bool{
		"read_one": true,
		"read_two": true,
	}

	unknown, omitted := diffRouteTools([]string{"read_one"}, offered)
	if len(unknown) != 0 {
		t.Fatalf("omitted read-only tool must not be flagged as unknown, got %v", unknown)
	}
	if len(omitted) != 1 || omitted[0] != "read_two" {
		t.Fatalf("omitted = %v, want [read_two]", omitted)
	}

	unknown, omitted = diffRouteTools([]string{"read_one", "renamed_tool"}, offered)
	if len(unknown) != 1 || unknown[0] != "renamed_tool" {
		t.Fatalf("unknown = %v, want [renamed_tool]", unknown)
	}
	if len(omitted) != 1 || omitted[0] != "read_two" {
		t.Fatalf("omitted = %v, want [read_two]", omitted)
	}
}

func checkRouteFileTools(t *testing.T, routeFile string, loaders map[string]upstreamToolLoader) {
	t.Helper()
	routes, err := loadPilotRoutes(routeFile)
	if err != nil {
		t.Fatalf("load %s: %v", routeFile, err)
	}
	cache := make(map[string]map[string]bool, len(loaders))
	for _, r := range routes {
		if r.PassAllTools {
			continue
		}
		loader, ok := loaders[r.App]
		if !ok {
			if _, statErr := os.Stat(filepath.Join("..", r.App)); statErr == nil {
				t.Errorf("%s route %s (%s): in-repo app %q has no tool loader in route_tools_test.go", routeFile, r.Route, r.Target, r.App)
			} else {
				t.Logf("%s route %s (%s): skipping external app %q", routeFile, r.Route, r.Target, r.App)
			}
			continue
		}
		offered, cached := cache[r.App]
		if !cached {
			offered, err = loader()
			if err != nil {
				t.Fatalf("%s route %s (%s): load upstream tools for %s: %v", routeFile, r.Route, r.Target, r.App, err)
			}
			cache[r.App] = offered
		}
		unknown, omitted := diffRouteTools(r.Tools, offered)
		for _, tool := range unknown {
			t.Errorf("%s route %s (%s): tool %q is not offered by %s", routeFile, r.Route, r.Target, tool, r.App)
		}
		if len(omitted) > 0 {
			t.Logf("%s route %s (%s): upstream tools omitted from route allowlist: %v", routeFile, r.Route, r.Target, omitted)
		}
	}
}

func diffRouteTools(routeTools []string, offered map[string]bool) (unknown, omitted []string) {
	listed := make(map[string]bool, len(routeTools))
	for _, name := range routeTools {
		listed[name] = true
		if !offered[name] {
			unknown = append(unknown, name)
		}
	}
	for name := range offered {
		if !listed[name] {
			omitted = append(omitted, name)
		}
	}
	sort.Strings(unknown)
	sort.Strings(omitted)
	return unknown, omitted
}

func parseFuncDecl(path, funcName string) (*ast.FuncDecl, error) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, path, nil, 0)
	if err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if ok && fn.Name.Name == funcName && fn.Body != nil {
			return fn, nil
		}
	}
	return nil, fmt.Errorf("function %s not found in %s", funcName, path)
}

func stringLitValue(expr ast.Expr) (string, bool) {
	lit, ok := expr.(*ast.BasicLit)
	if !ok || lit.Kind != token.STRING {
		return "", false
	}
	s, err := strconv.Unquote(lit.Value)
	return s, err == nil
}

func compositeStringField(cl *ast.CompositeLit, matchKey func(ast.Expr) bool) (string, bool) {
	for _, elt := range cl.Elts {
		kv, ok := elt.(*ast.KeyValueExpr)
		if !ok || !matchKey(kv.Key) {
			continue
		}
		return stringLitValue(kv.Value)
	}
	return "", false
}

func structStringField(cl *ast.CompositeLit, fieldName string) (string, bool) {
	return compositeStringField(cl, func(expr ast.Expr) bool {
		key, ok := expr.(*ast.Ident)
		return ok && key.Name == fieldName
	})
}

func mapStringKey(cl *ast.CompositeLit, keyName string) (string, bool) {
	return compositeStringField(cl, func(expr ast.Expr) bool {
		key, ok := stringLitValue(expr)
		return ok && key == keyName
	})
}

func onboardingToolNames(path string) (map[string]bool, error) {
	fn, err := parseFuncDecl(path, "handleListTools")
	if err != nil {
		return nil, err
	}
	tools := map[string]bool{}
	ast.Inspect(fn.Body, func(n ast.Node) bool {
		cl, ok := n.(*ast.CompositeLit)
		if !ok {
			return true
		}
		if arr, ok := cl.Type.(*ast.ArrayType); ok {
			if ident, ok := arr.Elt.(*ast.Ident); ok && ident.Name == "Tool" {
				for _, elt := range cl.Elts {
					item, ok := elt.(*ast.CompositeLit)
					if !ok {
						continue
					}
					if name, ok := structStringField(item, "Name"); ok && name != "" {
						tools[name] = true
					}
				}
			}
		} else if ident, ok := cl.Type.(*ast.Ident); ok && ident.Name == "Tool" {
			if name, ok := structStringField(cl, "Name"); ok && name != "" {
				tools[name] = true
			}
		}
		return true
	})
	if len(tools) == 0 {
		return nil, fmt.Errorf("no tools found in %s", path)
	}
	return tools, nil
}

func mapSliceToolNames(path, funcName string) (map[string]bool, error) {
	fn, err := parseFuncDecl(path, funcName)
	if err != nil {
		return nil, err
	}
	tools := map[string]bool{}
	ast.Inspect(fn.Body, func(n ast.Node) bool {
		cl, ok := n.(*ast.CompositeLit)
		if !ok {
			return true
		}
		arr, ok := cl.Type.(*ast.ArrayType)
		if !ok {
			return true
		}
		if _, ok := arr.Elt.(*ast.MapType); !ok {
			return true
		}
		for _, elt := range cl.Elts {
			item, ok := elt.(*ast.CompositeLit)
			if !ok {
				continue
			}
			if name, ok := mapStringKey(item, "name"); ok && name != "" {
				tools[name] = true
			}
		}
		return true
	})
	if len(tools) == 0 {
		return nil, fmt.Errorf("no tools found in %s", path)
	}
	return tools, nil
}

func dollyToolNames(path string) (map[string]bool, error) {
	fn, err := parseFuncDecl(path, "toolDefinitions")
	if err != nil {
		return nil, err
	}
	tools := map[string]bool{}
	ast.Inspect(fn.Body, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		fnIdent, ok := call.Fun.(*ast.Ident)
		if !ok || (fnIdent.Name != "tool" && fnIdent.Name != "writeTool") || len(call.Args) == 0 {
			return true
		}
		if name, ok := stringLitValue(call.Args[0]); ok && name != "" {
			tools[name] = true
		}
		return true
	})
	if len(tools) == 0 {
		return nil, fmt.Errorf("no tools found in %s", path)
	}
	return tools, nil
}
