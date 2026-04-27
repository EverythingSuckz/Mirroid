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
	StateIdle      ProcessState = iota // no process running
	StateLaunching                     // process started, waiting for video stream
	StateMirroring                     // video stream confirmed active (Texture: seen)
	StateError                         // process encountered an error
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

	// OnError is called when scrcpy emits an ERROR line on stdout/stderr.
	// Fires for every error (transient retries included), not just terminal ones.
	OnError func(serial, msg string)
}

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
			// detect scrcpy server errors in real-time
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
				// notify immediately so the table status updates
				if r.OnStateChange != nil {
					r.OnStateChange(serial)
				}
				if r.OnError != nil {
					r.OnError(serial, errMsg)
				}
			}
			// detect video stream active (scrcpy window opened)
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

		if err := scanner.Err(); err != nil {
			logFn(fmt.Sprintf("[scrcpy] stdout read error: %v", err))
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
		// keep StateError so the UI can show it briefly; it will be
		// auto-cleared by the next refreshDevices cycle (~3 s).
		if r.failedSerials[serial] != "" {
			r.deviceStates[serial] = StateError
		} else {
			delete(r.deviceStates, serial)
		}
		r.mu.Unlock()

		if r.OnStateChange != nil {
			r.OnStateChange(serial)
		}
	}()

	return nil
}

// CommandPreview returns the full command string that would be executed.
func (r *Runner) CommandPreview(serial string, opts model.ScrcpyOptions) string {
	return strings.Join(opts.BuildCommand(r.scrcpyPath, serial), " ")
}

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

func (r *Runner) RunningCount() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.processes)
}

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

func (r *Runner) StateFor(serial string) ProcessState {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.deviceStates[serial]
}

func (r *Runner) LastErrorFor(serial string) string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.failedSerials[serial]
}

func (r *Runner) ClearErrorFor(serial string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.failedSerials, serial)
	if r.deviceStates[serial] == StateError {
		delete(r.deviceStates, serial)
	}
}

func (r *Runner) ClearExitedErrors() {
	r.mu.Lock()
	defer r.mu.Unlock()

	// build a set of serials that still have a running process.
	running := make(map[string]bool, len(r.processes))
	for _, p := range r.processes {
		running[p.serial] = true
	}

	for serial, state := range r.deviceStates {
		if state == StateError && !running[serial] {
			delete(r.deviceStates, serial)
			delete(r.failedSerials, serial)
		}
	}
}
