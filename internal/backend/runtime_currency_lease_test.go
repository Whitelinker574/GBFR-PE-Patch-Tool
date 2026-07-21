package backend

import (
	"go/ast"
	"go/parser"
	"go/token"
	"testing"
)

func currencyAppFunctionBodies(t *testing.T) map[string]*ast.BlockStmt {
	t.Helper()
	parsed, err := parser.ParseFile(token.NewFileSet(), "app.go", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	bodies := make(map[string]*ast.BlockStmt)
	for _, decl := range parsed.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if ok && fn.Recv != nil && fn.Body != nil {
			bodies[fn.Name.Name] = fn.Body
		}
	}
	return bodies
}

func TestCurrencyMemoryEntrypointsAcquireStableProcessLease(t *testing.T) {
	bodies := currencyAppFunctionBodies(t)
	for _, name := range []string{"CurrencyGetAll", "CurrencySetOne"} {
		body := bodies[name]
		if body == nil {
			t.Fatalf("missing %s", name)
		}
		if !blockCallsSelector(body, "a", "acquireGameProcessLease") {
			t.Errorf("%s must pin hProcess/moduleBase/PID for its full operation", name)
		}
		if !blockCallsSelector(body, "liveMemoryWriteMu", "Lock") {
			t.Errorf("%s may install or update a runtime hook and must join the global live-memory transaction", name)
		}
	}
}

func TestOwnedCurrencyAndPotionWritesAcquireCurrentCharaLease(t *testing.T) {
	bodies := currencyAppFunctionBodies(t)
	for _, name := range []string{"CurrencySetOneOwned", "PotionSetOneOwned"} {
		body := bodies[name]
		if body == nil {
			t.Fatalf("missing %s", name)
		}
		if !blockCallsSelector(body, "a", "acquireOwnedRuntimeWriteLease") {
			t.Errorf("%s must reject stale page ownership before process IO", name)
		}
		if !blockCallsSelector(body, "liveMemoryWriteMu", "Lock") {
			t.Errorf("%s must join the global live-memory transaction", name)
		}
	}
}

func TestPotionEntrypointsPinProcessAndWritesUseTransaction(t *testing.T) {
	bodies := currencyAppFunctionBodies(t)
	for _, name := range []string{"PotionGetAll", "PotionSetOne"} {
		body := bodies[name]
		if body == nil || !blockCallsSelector(body, "a", "acquireGameProcessLease") {
			t.Errorf("%s must pin the process handle for its full operation", name)
		}
	}
	for _, name := range []string{"currencySetOneLocked", "potionSetOneLocked"} {
		body := bodies[name]
		if body == nil || !blockCallsSelector(body, "a", "writeInt32TransactionalLocked") {
			t.Errorf("%s must use verified write/rollback semantics", name)
		}
	}
}

func countCallsIdent(body *ast.BlockStmt, name string) int {
	count := 0
	ast.Inspect(body, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok {
			return true
		}
		ident, ok := call.Fun.(*ast.Ident)
		if ok && ident.Name == name {
			count++
		}
		return true
	})
	return count
}

func countCallsSelector(body *ast.BlockStmt, receiver, method string) int {
	count := 0
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
			count++
		}
		return true
	})
	return count
}

func TestCurrencySetOneRevalidatesRootAndValueAfterBackup(t *testing.T) {
	body := currencyAppFunctionBodies(t)["currencySetOneLocked"]
	if body == nil {
		t.Fatal("missing currencySetOneLocked")
	}
	if got := countCallsSelector(body, "a", "currencyRoot"); got < 2 {
		t.Fatalf("CurrencySetOne must read the inventory root before and after the slow backup, calls=%d", got)
	}
	if got := countCallsIdent(body, "readProcessMemory"); got < 2 {
		t.Fatalf("CurrencySetOne must snapshot and re-read the target value before writing, calls=%d", got)
	}
}
