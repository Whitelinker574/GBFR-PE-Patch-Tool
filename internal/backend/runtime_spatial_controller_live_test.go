package backend

import (
	"os"
	"testing"
	"time"
)

// This opt-in diagnostic is strictly read-only. It attaches through the same
// Chara owner lease used by the UI, samples only the bounded controller probe,
// and releases the process handle afterwards. Set the environment variable and
// move/jump the local character during the short capture window.
func TestRuntimeSpatialControllerLiveReadOnly203(t *testing.T) {
	if os.Getenv("GBFR_LIVE_SPATIAL_CONTROLLER_READONLY") != "1" {
		t.Skip("set GBFR_LIVE_SPATIAL_CONTROLLER_READONLY=1 while GAME 2.0.3 is running")
	}

	app := NewApp()
	info, err := app.CharaAcquire(1)
	if err != nil {
		t.Fatal(err)
	}
	if info.OwnerToken == "" || info.PID == 0 {
		t.Fatalf("read-only controller probe did not acquire an owned process: %+v", info)
	}
	t.Cleanup(func() {
		if err := app.CharaRelease(info.OwnerToken); err != nil {
			t.Errorf("read-only controller probe release: %v", err)
		}
	})

	for sample := 0; sample < 24; sample++ {
		result, err := app.RuntimeSpatialControllerProbeOwned(info.OwnerToken)
		if err != nil {
			t.Fatalf("sample %d: %v", sample, err)
		}
		if result.GameVersion != "2.0.3" || !result.RuntimeVerified || result.ControllerAddress == 0 {
			t.Fatalf("sample %d was not a verified 2.0.3 controller frame: %+v", sample, result)
		}
		values := make([]any, 0, len(result.Fields)*2+2)
		values = append(values, result.CurrentY)
		for _, field := range result.Fields {
			values = append(values, field.Offset, field.RawBytes)
		}
		t.Logf("sample=%03d y=%v fields(offset,raw)=%v", sample, result.CurrentY, values[1:])
		time.Sleep(20 * time.Millisecond)
	}
}
