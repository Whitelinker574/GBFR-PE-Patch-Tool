package backend

import (
	"go/ast"
	"go/token"
	"testing"

	"golang.org/x/sys/windows"
)

func TestCanReuseGameProcessRequiresSameLiveHandle(t *testing.T) {
	tests := []struct {
		name                       string
		cachedPID, discoveredPID   uint32
		hasHandle, hasModule, live bool
		want                       bool
	}{
		{"same live process", 100, 100, true, true, true, true},
		{"missing handle", 100, 100, false, true, true, false},
		{"missing module", 100, 100, true, false, true, false},
		{"dead cached handle", 100, 100, true, true, false, false},
		{"pid changed", 100, 101, true, true, true, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := canReuseGameProcess(tt.cachedPID, tt.discoveredPID, tt.hasHandle, tt.hasModule, tt.live); got != tt.want {
				t.Fatalf("canReuseGameProcess() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestLiveMemoryWritePoisonIsScopedToUncertainProcess(t *testing.T) {
	poisoned := processInstanceID{PID: 100, Created: 1000}
	if !liveMemoryWritePoisoned(poisoned, poisoned) {
		t.Fatal("same process must remain blocked after an indeterminate save")
	}
	if liveMemoryWritePoisoned(poisoned, processInstanceID{PID: 101, Created: 1001}) {
		t.Fatal("a newly discovered process must not inherit the old process poison")
	}
	if liveMemoryWritePoisoned(processInstanceID{}, poisoned) {
		t.Fatal("zero poison PID must allow writes")
	}
	if liveMemoryWritePoisoned(poisoned, processInstanceID{PID: poisoned.PID, Created: poisoned.Created + 1}) {
		t.Fatal("a new process reusing the old PID must not inherit its poison")
	}
}

func TestProcessInstanceIdentityIncludesCreationTime(t *testing.T) {
	base := processInstanceID{PID: 100, Created: 1234}
	if !sameProcessInstance(base, base) {
		t.Fatal("the same PID and creation time must identify one process instance")
	}
	if sameProcessInstance(base, processInstanceID{PID: 101, Created: base.Created}) {
		t.Fatal("different PIDs must identify different process instances")
	}
	if sameProcessInstance(base, processInstanceID{PID: base.PID, Created: base.Created + 1}) {
		t.Fatal("PID reuse with a different creation time must identify a new process instance")
	}
	if sameProcessInstance(processInstanceID{}, base) {
		t.Fatal("an incomplete process identity must never compare equal")
	}
}

func firstSelectorCallPosition(body *ast.BlockStmt, receiver, method string) token.Pos {
	position := token.NoPos
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
		if ok && ident.Name == receiver && (position == token.NoPos || call.Pos() < position) {
			position = call.Pos()
		}
		return true
	})
	return position
}

func firstIdentCallPosition(body *ast.BlockStmt, name string) token.Pos {
	position := token.NoPos
	ast.Inspect(body, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok {
			return true
		}
		ident, ok := call.Fun.(*ast.Ident)
		if ok && ident.Name == name && (position == token.NoPos || call.Pos() < position) {
			position = call.Pos()
		}
		return true
	})
	return position
}

func TestProcessPoisonIsNotClearedBeforeSuccessfulOpen(t *testing.T) {
	checks := []struct {
		name   string
		method string
	}{
		{name: "CharaAttach", method: "charaAttachLocked"},
		{name: "ensureGameProcess", method: "ensureGameProcessLocked"},
	}
	for _, check := range checks {
		t.Run(check.name, func(t *testing.T) {
			body := appMethodBodyInFile(t, "app.go", check.method)
			if body == nil {
				t.Fatalf("%s implementation %s not found", check.name, check.method)
			}
			clearPosition := firstSelectorCallPosition(body, "a", "clearLiveMemoryPoisonForNewProcess")
			openPosition := firstSelectorCallPosition(body, "windows", "OpenProcess")
			modulePosition := firstIdentCallPosition(body, "getModuleBase")
			creationPosition := firstIdentCallPosition(body, "processCreationTime")
			if clearPosition == token.NoPos || openPosition == token.NoPos || modulePosition == token.NoPos || creationPosition == token.NoPos {
				t.Fatalf("%s must open and identify a process before considering clearing poison", check.name)
			}
			if clearPosition < openPosition || clearPosition < modulePosition || clearPosition < creationPosition {
				t.Fatalf("%s clears poison before the replacement handle, module and creation time are all known", check.name)
			}
		})
	}
}

func TestCurrentProcessCreationTimeIsStable(t *testing.T) {
	first, err := processCreationTime(windows.CurrentProcess())
	if err != nil {
		t.Fatal(err)
	}
	second, err := processCreationTime(windows.CurrentProcess())
	if err != nil {
		t.Fatal(err)
	}
	if first == 0 || first != second {
		t.Fatalf("current process creation time changed: first=%d second=%d", first, second)
	}
}

func TestSuccessfulNewProcessIdentityClearsOnlyOldPoison(t *testing.T) {
	oldProcess := processInstanceID{PID: 100, Created: 1000}
	app := &App{liveMemoryIndeterminateProcess: oldProcess}
	app.clearLiveMemoryPoisonForNewProcess(oldProcess)
	if app.liveMemoryIndeterminateProcess != oldProcess {
		t.Fatal("reconnecting to the same process instance cleared its quarantine")
	}
	app.clearLiveMemoryPoisonForNewProcess(processInstanceID{PID: oldProcess.PID, Created: oldProcess.Created + 1})
	if app.liveMemoryIndeterminateProcess != (processInstanceID{}) {
		t.Fatalf("a successfully identified replacement process retained old poison: %+v", app.liveMemoryIndeterminateProcess)
	}
}

func TestPublicDetachJoinsGlobalLiveMemoryTransaction(t *testing.T) {
	body := appMethodBodyInFile(t, "app.go", "CharaDetach")
	if body == nil || !blockCallsSelector(body, "liveMemoryWriteMu", "Lock") {
		t.Fatal("CharaDetach must wait for in-flight live-memory writes before closing the shared process handle")
	}
}

func TestGenericRuntimeAttachDoesNotScanLegacyCharacterList(t *testing.T) {
	body := appMethodBodyInFile(t, "app.go", "charaAttachLocked")
	if body == nil {
		t.Fatal("charaAttachLocked not found")
	}
	if firstSelectorCallPosition(body, "a", "charaManager") != token.NoPos {
		t.Fatal("generic runtime attach must not scan the legacy character list before returning")
	}
}
