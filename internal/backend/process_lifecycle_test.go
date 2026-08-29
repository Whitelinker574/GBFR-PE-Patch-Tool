package backend

import (
	"go/ast"
	"go/parser"
	"go/token"
	"strings"
	"testing"

	"golang.org/x/sys/windows"
)

func functionBodyInFile(t *testing.T, path, name string) *ast.BlockStmt {
	t.Helper()
	parsed, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	for _, declaration := range parsed.Decls {
		function, ok := declaration.(*ast.FuncDecl)
		if ok && function.Recv == nil && function.Name.Name == name {
			return function.Body
		}
	}
	return nil
}

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
	openHelper := functionBodyInFile(t, "app.go", "openVerifiedLegacyRuntimeProcess")
	if openHelper == nil ||
		firstSelectorCallPosition(openHelper, "windows", "OpenProcess") == token.NoPos ||
		firstIdentCallPosition(openHelper, "verifyLegacyRuntimeExecutableHandle") == token.NoPos ||
		firstIdentCallPosition(openHelper, "getModuleBase") == token.NoPos ||
		firstIdentCallPosition(openHelper, "processCreationTime") == token.NoPos {
		t.Fatal("shared process helper must open, verify and fully identify the replacement process")
	}
	publishHelper := appMethodBodyInFile(t, "app.go", "publishVerifiedGameProcess")
	if publishHelper == nil || firstSelectorCallPosition(publishHelper, "a", "clearLiveMemoryPoisonForNewProcess") == token.NoPos {
		t.Fatal("verified process publication must clear poison only after accepting a complete identity")
	}
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
			openPosition := firstIdentCallPosition(body, "openVerifiedLegacyRuntimeProcess")
			publishPosition := firstSelectorCallPosition(body, "a", "publishVerifiedGameProcess")
			if openPosition == token.NoPos || publishPosition == token.NoPos || publishPosition < openPosition {
				t.Fatalf("%s must fully open and identify a process before publishing and clearing poison", check.name)
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

func TestKnownRuntimeExecutableDigestsIncludeGame202Through204(t *testing.T) {
	for _, digest := range []string{runtimePatchCatalogGameSHA256, game203ExecutableSHA256, game204ExecutableSHA256, game205ExecutableSHA256} {
		if !isSupportedRuntimeExecutableDigest(digest) {
			t.Fatalf("known game executable %s was rejected by the shared runtime attach boundary", digest)
		}
	}
	if isSupportedRuntimeExecutableDigest(strings.Repeat("0", 64)) {
		t.Fatal("unknown game executable was accepted by the shared runtime attach boundary")
	}
}

func TestLegacyRuntimeExecutableErrorRejectsUnknownBuild(t *testing.T) {
	message := legacyRuntimeExecutableError("实时功能", strings.Repeat("0", 64)).Error()
	if !strings.Contains(message, "仅支持已识别的游戏 2.0.2 / 2.0.3 / 2.0.4 / 2.0.5") ||
		!strings.Contains(message, "不会连接或写入") {
		t.Fatalf("unknown executable message does not fail closed clearly: %q", message)
	}
}

func TestSharedRuntimeAttachVerifiesExecutableBeforePublishingConnection(t *testing.T) {
	helper := functionBodyInFile(t, "app.go", "openVerifiedLegacyRuntimeProcess")
	if helper == nil {
		t.Fatal("openVerifiedLegacyRuntimeProcess not found")
	}
	if firstIdentCallPosition(helper, "verifyLegacyRuntimeExecutableHandle") == token.NoPos {
		t.Fatal("shared runtime attach helper does not verify the executable")
	}
	for _, method := range []string{"charaAttachLocked", "ensureGameProcessLocked"} {
		body := appMethodBodyInFile(t, "app.go", method)
		if body == nil {
			t.Fatalf("%s not found", method)
		}
		openPosition := firstIdentCallPosition(body, "openVerifiedLegacyRuntimeProcess")
		if openPosition == token.NoPos {
			t.Fatalf("%s bypasses the verified shared attach helper", method)
		}
		publishPosition := firstSelectorCallPosition(body, "a", "publishVerifiedGameProcess")
		if publishPosition == token.NoPos || openPosition > publishPosition {
			t.Fatalf("%s publishes the connection before verified open succeeds", method)
		}
	}
}
