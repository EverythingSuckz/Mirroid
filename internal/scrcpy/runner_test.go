package scrcpy

import (
	"os/exec"
	"testing"
)

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

func TestClearExitedErrors_ClearsOnlyExitedErrors(t *testing.T) {
	r := NewRunner("")

	// dev1: exited with error (no running process) — should be cleared
	r.mu.Lock()
	r.failedSerials["dev1"] = "connection refused"
	r.deviceStates["dev1"] = StateError
	r.mu.Unlock()

	// dev2: still running with error (e.g., non-fatal audio error) — must NOT be cleared
	cmd := exec.Command("echo") // dummy process
	r.mu.Lock()
	r.processes = append(r.processes, &Process{cmd: cmd, serial: "dev2"})
	r.failedSerials["dev2"] = "audio error"
	r.deviceStates["dev2"] = StateError
	r.mu.Unlock()

	// dev3: actively mirroring — must NOT be touched
	r.mu.Lock()
	r.deviceStates["dev3"] = StateMirroring
	r.mu.Unlock()

	r.ClearExitedErrors()

	// dev1 should be fully cleared
	if got := r.StateFor("dev1"); got != StateIdle {
		t.Errorf("dev1: StateFor = %d, want StateIdle (%d)", got, StateIdle)
	}
	if got := r.LastErrorFor("dev1"); got != "" {
		t.Errorf("dev1: LastErrorFor = %q, want empty", got)
	}

	// dev2 still has a process — error should persist
	if got := r.StateFor("dev2"); got != StateError {
		t.Errorf("dev2: StateFor = %d, want StateError (%d)", got, StateError)
	}
	if got := r.LastErrorFor("dev2"); got != "audio error" {
		t.Errorf("dev2: LastErrorFor = %q, want %q", got, "audio error")
	}

	// dev3 is mirroring — untouched
	if got := r.StateFor("dev3"); got != StateMirroring {
		t.Errorf("dev3: StateFor = %d, want StateMirroring (%d)", got, StateMirroring)
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
