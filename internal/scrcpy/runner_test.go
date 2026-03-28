package scrcpy

import "testing"

func TestStateFor_DefaultIsIdle(t *testing.T) {
	r := NewRunner("")

	// Unknown serials must return StateIdle (zero value).
	// The UI status switch depends on this — if the zero value is wrong,
	// every device shows the wrong status.
	got := r.StateFor("unknown-serial")
	if got != StateIdle {
		t.Errorf("StateFor(unknown) = %d, want StateIdle (%d)", got, StateIdle)
	}

	// Verify StateIdle is actually the zero value of ProcessState.
	var zero ProcessState
	if zero != StateIdle {
		t.Errorf("zero value of ProcessState = %d, want StateIdle (%d)", zero, StateIdle)
	}
}

func TestClearErrorFor_ClearsDeviceState(t *testing.T) {
	r := NewRunner("")

	// Simulate a device in StateError with a stored error message.
	r.mu.Lock()
	r.failedSerials["dev1"] = "connection refused"
	r.deviceStates["dev1"] = StateError
	r.mu.Unlock()

	r.ClearErrorFor("dev1")

	// After clearing, the device should be back to StateIdle.
	if got := r.StateFor("dev1"); got != StateIdle {
		t.Errorf("after ClearErrorFor: StateFor(dev1) = %d, want StateIdle (%d)", got, StateIdle)
	}
	if got := r.LastErrorFor("dev1"); got != "" {
		t.Errorf("after ClearErrorFor: LastErrorFor(dev1) = %q, want empty", got)
	}
}

func TestClearErrorFor_DoesNotClearNonErrorState(t *testing.T) {
	r := NewRunner("")

	// Simulate a device that is actively mirroring but also has a stale
	// error message (e.g., non-fatal audio error before Texture: appeared).
	r.mu.Lock()
	r.failedSerials["dev1"] = "audio codec not supported"
	r.deviceStates["dev1"] = StateMirroring
	r.mu.Unlock()

	r.ClearErrorFor("dev1")

	// The error message should be cleared, but the device state must
	// remain StateMirroring — clearing an error tooltip must not break
	// an active mirroring session.
	if got := r.StateFor("dev1"); got != StateMirroring {
		t.Errorf("after ClearErrorFor: StateFor(dev1) = %d, want StateMirroring (%d)", got, StateMirroring)
	}
	if got := r.LastErrorFor("dev1"); got != "" {
		t.Errorf("after ClearErrorFor: LastErrorFor(dev1) = %q, want empty", got)
	}
}
