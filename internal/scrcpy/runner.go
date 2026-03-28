package scrcpy

import (
	"bufio"
	"fmt"
	"os/exec"
	"strings"
	"sync"

	"mirroid/internal/model"
	"mirroid/internal/platform"
)

// ProcessState represents the lifecycle state of a scrcpy process.
type ProcessState int

const (
	StateIdle       ProcessState = iota // no process running
	StateLaunching                      // process started, waiting for video stream
	StateMirroring                      // video stream confirmed active (Texture: seen)
	StateError                          // process encountered an error
)

// Process wraps a running scrcpy instance.
type Process struct {
	cmd    *exec.Cmd
	serial string
}

// Runner manages scrcpy processes.
type Runner struct {
	scrcpyPath string
	mu         sync.Mutex
	processes  []*Process

	// deviceStates tracks the current lifecycle state for each device serial.
	deviceStates map[string]ProcessState

	// failedSerials tracks the last error message for devices whose scrcpy
	// process exited with an error. Cleared on next successful launch.
	failedSerials map[string]string

	// OnStateChange is called after the process list changes (launch, exit).
	// The serial of the affected device is passed as argument.
	OnStateChange func(serial string)
}

// NewRunner creates a scrcpy runner.
func NewRunner(scrcpyPath string) *Runner {
	if scrcpyPath == "" {
		scrcpyPath = "scrcpy"
	}
	return &Runner{
		scrcpyPath:    scrcpyPath,
		deviceStates:  make(map[string]ProcessState),
		failedSerials: make(map[string]string),
	}
}

// Launch starts a scrcpy process for the given device and options.
// If windowTitle is non-empty, scrcpy is launched with --window-title for reparenting.
// Log lines are sent to logFn from a goroutine (thread-safe callback expected).
func (r *Runner) Launch(serial string, opts model.ScrcpyOptions, logFn func(string), windowTitle string) error {
	args := opts.BuildCommand(r.scrcpyPath, serial)
	if windowTitle != "" {
		args = append(args, "--window-title", windowTitle)
	}

	cmd := exec.Command(args[0], args[1:]...)
	// suppress console window but keep the scrcpy GUI visible
	platform.SuppressConsole(cmd)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("scrcpy stdout pipe: %w", err)
	}
	cmd.Stderr = cmd.Stdout // merge stderr into stdout

	if err := cmd.Start(); err != nil {
		stdout.Close()
		return fmt.Errorf("scrcpy start: %w", err)
	}

	proc := &Process{cmd: cmd, serial: serial}
	r.mu.Lock()
	r.processes = append(r.processes, proc)
	delete(r.failedSerials, serial) // clear any previous error
	r.deviceStates[serial] = StateLaunching
	r.mu.Unlock()

	if r.OnStateChange != nil {
		r.OnStateChange(serial)
	}

	// read output in a goroutine
	go func() {
		scanner := bufio.NewScanner(stdout)
		for scanner.Scan() {
			line := scanner.Text()
			logFn(line)
			// Detect scrcpy server errors in real-time
			if strings.Contains(line, "ERROR") {
				var errMsg string
				if idx := strings.Index(line, "ERROR:"); idx >= 0 {
					errMsg = strings.TrimSpace(line[idx+len("ERROR:"):])
				} else {
					errMsg = line
				}
				r.mu.Lock()
				r.failedSerials[serial] = errMsg
				r.deviceStates[serial] = StateError
				r.mu.Unlock()
				// Notify immediately so the table status updates
				if r.OnStateChange != nil {
					r.OnStateChange(serial)
				}
			}
			// Detect video stream active (scrcpy window opened)
			if strings.Contains(line, "Texture:") {
				r.mu.Lock()
				if r.deviceStates[serial] == StateLaunching || r.deviceStates[serial] == StateError {
					r.deviceStates[serial] = StateMirroring
				}
				r.mu.Unlock()
				if r.OnStateChange != nil {
					r.OnStateChange(serial)
				}
			}
		}

		// wait for process exit
		exitErr := cmd.Wait()
		if exitErr != nil {
			logFn(fmt.Sprintf("[scrcpy] process for %s exited with error: %v", serial, exitErr))
		} else {
			logFn(fmt.Sprintf("[scrcpy] process for %s exited (code %d)", serial, cmd.ProcessState.ExitCode()))
		}

		// remove from tracked list and transition state
		r.mu.Lock()
		for i, p := range r.processes {
			if p == proc {
				r.processes = append(r.processes[:i], r.processes[i+1:]...)
				break
			}
		}
		delete(r.deviceStates, serial)
		r.mu.Unlock()

		if r.OnStateChange != nil {
			r.OnStateChange(serial)
		}
	}()

	return nil
}

// CommandPreview returns the full command string that would be executed.
func (r *Runner) CommandPreview(serial string, opts model.ScrcpyOptions) string {
	args := opts.BuildCommand(r.scrcpyPath, serial)
	result := ""
	for i, arg := range args {
		if i > 0 {
			result += " "
		}
		result += arg
	}
	return result
}

// StopAll terminates all running scrcpy processes gracefully, then kills stragglers.
func (r *Runner) StopAll() {
	r.mu.Lock()
	procs := make([]*Process, len(r.processes))
	copy(procs, r.processes)
	r.mu.Unlock()

	for _, proc := range procs {
		if proc.cmd.Process != nil {
			// it's not murder if it's a process
			_ = proc.cmd.Process.Kill()
		}
	}
}

// StopFor terminates scrcpy processes for a specific device serial.
func (r *Runner) StopFor(serial string) {
	r.mu.Lock()
	var toKill []*Process
	for _, p := range r.processes {
		if p.serial == serial {
			toKill = append(toKill, p)
		}
	}
	r.mu.Unlock()

	for _, proc := range toKill {
		if proc.cmd.Process != nil {
			_ = proc.cmd.Process.Kill()
		}
	}
}

// RunningCount returns the number of running scrcpy processes.
func (r *Runner) RunningCount() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.processes)
}

// IsRunningFor returns true if scrcpy is currently running for the given serial.
func (r *Runner) IsRunningFor(serial string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, p := range r.processes {
		if p.serial == serial {
			return true
		}
	}
	return false
}

// StateFor returns the current process state for a device serial.
// Returns StateIdle if no process is tracked.
func (r *Runner) StateFor(serial string) ProcessState {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.deviceStates[serial]
}

// LastErrorFor returns the last error message for a device, or empty if none.
func (r *Runner) LastErrorFor(serial string) string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.failedSerials[serial]
}

// ClearErrorFor removes the stored error for a device.
func (r *Runner) ClearErrorFor(serial string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.failedSerials, serial)
	if r.deviceStates[serial] == StateError {
		delete(r.deviceStates, serial)
	}
}
