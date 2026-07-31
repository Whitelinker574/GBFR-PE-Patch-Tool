package backend

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestRuntimeCompanionCrashRestoresLiveGame(t *testing.T) {
	if os.Getenv("GBFR_LIVE_RUNTIME_CRASH_TEST") != "1" {
		t.Skip("set GBFR_LIVE_RUNTIME_CRASH_TEST=1 for the explicit live-game crash recovery acceptance")
	}
	readyPath := os.Getenv("GBFR_RUNTIME_CRASH_READY")
	if os.Getenv("GBFR_RUNTIME_CRASH_CHILD") == "1" {
		if readyPath == "" {
			t.Fatal("crash-test child is missing its ready path")
		}
		triggerPath := os.Getenv("GBFR_RUNTIME_CRASH_TRIGGER")
		if triggerPath == "" {
			t.Fatal("crash-test child is missing its trigger path")
		}
		app := &App{}
		if _, err := app.DeployCameraMod(CameraDeployRequest{MaxDistance: 6, TargetHeight: 1.8, ZoomStep: 0.02}); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(readyPath, []byte("active"), 0o600); err != nil {
			t.Fatal(err)
		}
		for {
			if _, err := os.Stat(triggerPath); err == nil {
				// os.Exit deliberately bypasses Go defers and App shutdown
				// hooks, reproducing an abrupt desktop-process loss while the
				// game and injected native runtime remain alive.
				os.Exit(97)
			}
			time.Sleep(10 * time.Millisecond)
		}
	}

	if _, err := findRuntimeProcessInstance(); err != nil {
		t.Skipf("live game is not running: %v", err)
	}
	readyPath = filepath.Join(t.TempDir(), "runtime-active")
	triggerPath := filepath.Join(filepath.Dir(readyPath), "terminate-without-cleanup")
	child := exec.Command(os.Args[0], "-test.run=^TestRuntimeCompanionCrashRestoresLiveGame$", "-test.v")
	child.Env = append(os.Environ(),
		"GBFR_LIVE_RUNTIME_CRASH_TEST=1",
		"GBFR_RUNTIME_CRASH_CHILD=1",
		"GBFR_RUNTIME_CRASH_READY="+readyPath,
		"GBFR_RUNTIME_CRASH_TRIGGER="+triggerPath,
	)
	output := &strings.Builder{}
	child.Stdout = output
	child.Stderr = output
	if err := child.Start(); err != nil {
		t.Fatal(err)
	}
	childDone := make(chan error, 1)
	go func() { childDone <- child.Wait() }()
	childStopped := false
	defer func() {
		if !childStopped && child.Process != nil {
			_ = child.Process.Kill()
			<-childDone
		}
	}()
	deadline := time.Now().Add(12 * time.Second)
	for {
		select {
		case err := <-childDone:
			childStopped = true
			t.Fatalf("runtime child exited before activation: %v\n%s", err, output.String())
		default:
		}
		if _, err := os.Stat(readyPath); err == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("runtime child did not become active: %s", output.String())
		}
		time.Sleep(50 * time.Millisecond)
	}
	status := readRuntimeCompanionStatus("camera")
	if status.State != "active" {
		t.Fatalf("camera status before crash = %#v; child output: %s", status, output.String())
	}
	if err := os.WriteFile(triggerPath, []byte("exit"), 0o600); err != nil {
		t.Fatal(err)
	}
	waitErr := <-childDone
	childStopped = true
	if waitErr == nil || child.ProcessState == nil || child.ProcessState.ExitCode() != 97 {
		t.Fatalf("runtime child did not terminate through the abrupt-exit path: %v\n%s", waitErr, output.String())
	}

	app := &App{}
	deadline = time.Now().Add(5 * time.Second)
	var takeoverErr error
	for {
		_, takeoverErr = app.DeployCameraMod(CameraDeployRequest{MaxDistance: 6, TargetHeight: 1.8, ZoomStep: 0.02})
		if takeoverErr == nil {
			break
		}
		status = readRuntimeCompanionStatus("camera")
		if status.State == "restore_failed" || time.Now().After(deadline) {
			t.Fatalf("new tool instance could not safely take over immediately after crash: %v; status=%#v child=%s", takeoverErr, status, output.String())
		}
		time.Sleep(25 * time.Millisecond)
	}
	process, err := findRuntimeProcessInstance()
	if err != nil {
		t.Fatal(err)
	}
	lease, ok := app.runtimeCompanionLease("camera", process)
	if !ok {
		t.Fatal("takeover did not retain a generation-scoped camera lease")
	}
	time.Sleep(500 * time.Millisecond)
	status = readRuntimeCompanionStatus("camera")
	owner := readRuntimeCompanionOwner("camera")
	if status.State != "active" || status.Generation != lease.Generation ||
		owner.Generation != lease.Generation || !app.ownsRuntimeCompanion("camera", process) {
		t.Fatalf("old native runtime overwrote the immediate takeover: status=%#v owner=%#v lease=%#v", status, owner, lease)
	}
	if err := app.RemoveCameraMod(""); err != nil {
		t.Fatalf("new tool instance could not restore its verification runtime: %v", err)
	}
}
