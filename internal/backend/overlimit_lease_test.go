package backend

import (
	"go/ast"
	"go/parser"
	"go/token"
	"testing"
)

func overLimitFunctionBodies(t *testing.T) map[string]*ast.BlockStmt {
	t.Helper()
	parsed, err := parser.ParseFile(token.NewFileSet(), "overlimit.go", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	bodies := make(map[string]*ast.BlockStmt)
	for _, decl := range parsed.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Recv == nil || fn.Body == nil {
			continue
		}
		bodies[fn.Name.Name] = fn.Body
	}
	return bodies
}

func blockCallsSelector(body *ast.BlockStmt, receiver, method string) bool {
	found := false
	ast.Inspect(body, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok {
			return true
		}
		selector, ok := call.Fun.(*ast.SelectorExpr)
		if !ok || selector.Sel.Name != method {
			return true
		}
		ident, ok := selector.X.(*ast.Ident)
		if ok && ident.Name == receiver {
			found = true
			return false
		}
		return true
	})
	return found
}

func TestOverLimitMemoryEntrypointsAcquireStableProcessLease(t *testing.T) {
	bodies := overLimitFunctionBodies(t)
	for _, check := range []struct{ name, implementation, lease string }{
		{name: "OverLimitScan", implementation: "OverLimitScan", lease: "acquireGameProcessLease"},
		{name: "OverLimitGetStatus", implementation: "OverLimitGetStatus", lease: "acquireGameProcessLease"},
		{name: "OverLimitEnable", implementation: "OverLimitEnable", lease: "acquireLegacyRuntimeMutationLease"},
		{name: "OverLimitDisable", implementation: "OverLimitDisable", lease: "acquireLegacyRuntimeMutationLease"},
		{name: "OverLimitSetSlot", implementation: "OverLimitSetSlot", lease: "acquireLegacyRuntimeMutationLease"},
		{name: "OverLimitSetAll", implementation: "overLimitSetAll", lease: "acquireLegacyRuntimeMutationLease"},
		{name: "OverLimitCommit", implementation: "OverLimitCommit", lease: "acquireGameProcessLease"},
	} {
		body := bodies[check.implementation]
		if body == nil {
			t.Fatalf("missing %s implementation %s", check.name, check.implementation)
		}
		if !blockCallsSelector(body, "a", check.lease) {
			t.Errorf("%s must pin hProcess/moduleBase/PID for its full operation", check.name)
		}
	}
	acquire := bodies["OverLimitAcquire"]
	if acquire == nil || !blockCallsSelector(acquire, "a", "acquireOwnedGameProcessLease") {
		t.Error("OverLimitAcquire must validate the global owner generation before pinning the process")
	}
	write := bodies["overLimitSetAll"]
	if write == nil || !blockCallsSelector(write, "a", "acquireOwnedRuntimeWriteLease") {
		t.Error("OverLimitSetAllOwned must keep its owner validation inside the stable process lease")
	}
}

func TestOverLimitWritesJoinGlobalLiveMemoryTransaction(t *testing.T) {
	bodies := overLimitFunctionBodies(t)
	for _, check := range []struct{ name, implementation string }{
		{name: "OverLimitSetSlot", implementation: "OverLimitSetSlot"},
		{name: "OverLimitSetAll", implementation: "overLimitSetAll"},
	} {
		body := bodies[check.implementation]
		if body == nil || !blockCallsSelector(body, "liveMemoryWriteMu", "Lock") {
			t.Errorf("%s must serialize with sigil, blessing, and summon writes", check.name)
		}
	}
}
