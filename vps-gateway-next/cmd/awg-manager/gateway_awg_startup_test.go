package main

import (
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"reflect"
	"testing"
)

// A running owned AWG child must remain reachable through runWithCleanup when
// local HTTP setup fails. os.Exit bypasses that mandatory process join.
func TestLocalServerStartupDoesNotBypassOwnedAWGCleanup(t *testing.T) {
	file, err := parser.ParseFile(token.NewFileSet(), "wiring_local.go", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, declaration := range file.Decls {
		fn, ok := declaration.(*ast.FuncDecl)
		if !ok || fn.Name.Name != "setupLocalServer" {
			continue
		}
		found = true
		ast.Inspect(fn.Body, func(node ast.Node) bool {
			call, ok := node.(*ast.CallExpr)
			if !ok {
				return true
			}
			selector, ok := call.Fun.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			ident, ok := selector.X.(*ast.Ident)
			if ok && ident.Name == "os" && selector.Sel.Name == "Exit" {
				t.Error("local server startup bypasses owned AWG cleanup through os.Exit")
			}
			return true
		})
		if fn.Type.Results == nil || len(fn.Type.Results.List) != 1 {
			t.Error("local server startup must return its failure to runWithCleanup")
		}
	}
	if !found {
		t.Fatal("local server setup definition missing")
	}
}

func TestLocalServerFailureReturnsThroughOwnedAWGCleanup(t *testing.T) {
	want := errors.New("local server setup failed")
	var order []string
	err := runWithCleanup(func() error {
		return runComposition(deploymentModeSingbox, compositionHooks{
			core: func() { order = append(order, "core") },
			local: func() error { order = append(order, "AWG-started"); return nil },
			localServer: func() error { order = append(order, "server-failed"); return want },
			localLifecycle: func() { t.Fatal("lifecycle after failed server setup") },
			serve: func() { t.Fatal("serve after failed server setup") },
		})
	}, func() { order = append(order, "owned-AWG-joined") })
	if !errors.Is(err, want) {
		t.Fatalf("server startup failure not preserved: %v", err)
	}
	if expected := []string{"core", "AWG-started", "server-failed", "owned-AWG-joined"}; !reflect.DeepEqual(order, expected) {
		t.Fatalf("cleanup ordering=%v, expected=%v", order, expected)
	}
}
