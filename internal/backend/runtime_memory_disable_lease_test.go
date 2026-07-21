package backend

import (
	"go/ast"
	"go/parser"
	"go/token"
	"testing"
)

func appMethodBodyInFile(t *testing.T, path, name string) *ast.BlockStmt {
	t.Helper()
	parsed, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	for _, decl := range parsed.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if ok && fn.Recv != nil && fn.Name.Name == name {
			return fn.Body
		}
	}
	return nil
}

func TestSigilAndWrightstoneDisablePinProcessBeforeLifecycleLock(t *testing.T) {
	for _, check := range []struct {
		file string
		name string
	}{
		{file: "sigil_memory.go", name: "SigilMemoryDisable"},
		{file: "wrightstone_memory.go", name: "WrightstoneMemoryDisable"},
	} {
		t.Run(check.name, func(t *testing.T) {
			body := appMethodBodyInFile(t, check.file, check.name)
			if body == nil {
				t.Fatalf("missing %s", check.name)
			}
			if !blockCallsSelector(body, "a", "acquireLegacyRuntimeMutationLease") {
				t.Fatalf("%s must pin hProcess/moduleBase/PID while detaching its hook", check.name)
			}
		})
	}
}

func TestLiveMemoryIndeterminatePathsPoisonProcessInstance(t *testing.T) {
	for _, check := range []struct {
		file string
		name string
	}{
		// Public compatibility and owned entry points now converge on these
		// shared implementations. Inspect the functions that own the complete
		// read/write/rollback critical section rather than their thin wrappers.
		{file: "sigil_memory.go", name: "sigilMemoryUpdate"},
		{file: "wrightstone_memory.go", name: "wrightstoneMemoryUpdate"},
		{file: "summon_memory.go", name: "summonUpdate"},
		{file: "overlimit.go", name: "callRemoteOneArg"},
	} {
		t.Run(check.name, func(t *testing.T) {
			body := appMethodBodyInFile(t, check.file, check.name)
			if body == nil {
				t.Fatalf("missing %s", check.name)
			}
			if !blockCallsSelector(body, "a", "poisonCurrentLiveMemoryWrites") {
				t.Fatalf("%s must quarantine the process instance when rollback cannot be proven", check.name)
			}
		})
	}
}
