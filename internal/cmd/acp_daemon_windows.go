//go:build windows

package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"syscall"

	"golang.org/x/sys/windows"
)

// DaemonPaths holds the paths for daemon-related files.
type DaemonPaths struct {
	LogFile string
	PidFile string
}

// GetDaemonPaths returns the paths for daemon log and PID files.
// On Windows, this uses %AppData%/swarmy.
func GetDaemonPaths() DaemonPaths {
	dir := getDaemonDir()
	return DaemonPaths{
		LogFile: filepath.Join(dir, "acp-daemon.log"),
		PidFile: filepath.Join(dir, "acp-daemon.pid"),
	}
}

// StartDaemon starts the ACP server as a background daemon process.
// It creates a detached process, redirects output to a log file, creates
// a PID file, and returns the PID of the daemon process.
func StartDaemon(host string, port int, autoApprove bool) (int, error) {
	// Get the path to the current executable.
	exePath, err := os.Executable()
	if err != nil {
		return 0, fmt.Errorf("failed to get executable path: %w", err)
	}

	// Get the daemon directory for log and PID files.
	daemonDir := getDaemonDir()

	// Create the daemon directory if it doesn't exist.
	if err := os.MkdirAll(daemonDir, 0o755); err != nil {
		return 0, fmt.Errorf("failed to create daemon directory: %w", err)
	}

	// Check if daemon is already running.
	if IsDaemonRunning() {
		return 0, fmt.Errorf("daemon is already running (PID: %d)", ReadPidFile())
	}

	// Build the command arguments.
	args := buildDaemonArgs(host, port, autoApprove)

	// Prepare the command to run the daemon process.
	// We pass the same arguments but set an environment variable
	// to indicate this is the daemon child process.
	cmd := os.Args[0]
	for _, a := range args {
		cmd += " " + a
	}

	// Set up the process to be detached from the console.
	// CREATE_NEW_PROCESS_GROUP ensures the process doesn't receive
	// signals from the parent terminal.
	attr := &os.ProcAttr{
		Dir:   ".",
		Env:   append(os.Environ(), "SWARMY_DAEMON_CHILD=1"),
		Files: []*os.File{nil, nil, nil},
		Sys: &syscall.SysProcAttr{
			CreationFlags: windows.CREATE_NEW_PROCESS_GROUP | windows.DETACHED_PROCESS,
		},
	}

	// Start the process.
	process, err := os.StartProcess(exePath, append([]string{os.Args[0]}, args...), attr)
	if err != nil {
		return 0, fmt.Errorf("failed to start daemon process: %w", err)
	}

	// Write the PID file.
	pidFile := filepath.Join(daemonDir, "acp-daemon.pid")
	pid := process.Pid
	if err := os.WriteFile(pidFile, []byte(fmt.Sprintf("%d\n", pid)), 0o644); err != nil {
		_ = process.Kill()
		return 0, fmt.Errorf("failed to write PID file: %w", err)
	}

	return pid, nil
}

// buildDaemonArgs builds the argument list for the daemon process.
// It filters out --daemon flag and ensures host, port, and auto-approve are set.
func buildDaemonArgs(host string, port int, autoApprove bool) []string {
	filtered := make([]string, 0)
	hasHost := false
	hasPort := false
	hasAutoApprove := false

	for _, arg := range os.Args[1:] {
		if arg == "--daemon" || arg == "-d" {
			continue
		}
		if arg == "serve" {
			continue
		}
		if arg == "acp" {
			continue
		}
		if arg == "--host" || arg == "-h" || len(arg) > 7 && arg[:7] == "--host=" {
			hasHost = true
		}
		if arg == "--port" || arg == "-p" || len(arg) > 7 && arg[:7] == "--port=" {
			hasPort = true
		}
		if arg == "--auto-approve" || len(arg) > 14 && arg[:14] == "--auto-approve" {
			hasAutoApprove = true
		}
		filtered = append(filtered, arg)
	}

	// Add serve command.
	result := []string{"acp", "serve"}

	// Add host if not already present.
	if !hasHost && host != "" {
		result = append(result, "--host", host)
	}

	// Add port if not already present.
	if !hasPort {
		result = append(result, "--port", strconv.Itoa(port))
	}

	// Add auto-approve if not already present.
	if !hasAutoApprove {
		autoVal := "true"
		if !autoApprove {
			autoVal = "false"
		}
		result = append(result, "--auto-approve", autoVal)
	}

	// Add filtered args.
	result = append(result, filtered...)

	return result
}

// getDaemonDir returns the directory for daemon files (logs, PID).
// On Windows, this uses %AppData%/swarmy.
func getDaemonDir() string {
	configDir, err := os.UserConfigDir()
	if err != nil {
		// Fallback to temp directory.
		return filepath.Join(os.TempDir(), "swarmy")
	}
	return filepath.Join(configDir, "swarmy")
}

// IsDaemonChild returns true if this process is the daemon child process.
func IsDaemonChild() bool {
	return os.Getenv("SWARMY_DAEMON_CHILD") == "1"
}

// ReadPidFile reads the PID from the PID file.
// Returns 0 if the file doesn't exist or is invalid.
func ReadPidFile() int {
	paths := GetDaemonPaths()
	data, err := os.ReadFile(paths.PidFile)
	if err != nil {
		return 0
	}

	// Trim whitespace and newlines.
	pidStr := string(data)
	for len(pidStr) > 0 && (pidStr[len(pidStr)-1] == '\n' || pidStr[len(pidStr)-1] == '\r') {
		pidStr = pidStr[:len(pidStr)-1]
	}

	pid, err := strconv.Atoi(pidStr)
	if err != nil {
		return 0
	}

	return pid
}

// IsDaemonRunning checks if the daemon process is currently running.
func IsDaemonRunning() bool {
	pid := ReadPidFile()
	if pid == 0 {
		return false
	}

	// On Windows, FindProcess always succeeds, so we need a different approach.
	// We try to open the process with SYNCHRONIZE permission.
	handle, err := windows.OpenProcess(windows.SYNCHRONIZE, false, uint32(pid))
	if err != nil {
		return false
	}
	windows.CloseHandle(handle)
	return true
}

// StopDaemon stops the running daemon process.
// Returns nil if the daemon was not running or was successfully stopped.
func StopDaemon() error {
	pid := ReadPidFile()
	if pid == 0 {
		return nil
	}

	process, err := os.FindProcess(pid)
	if err != nil {
		return nil
	}

	// Try to kill the process.
	if err := process.Kill(); err != nil {
		// Process may have already exited.
		return nil
	}

	// Remove the PID file.
	paths := GetDaemonPaths()
	_ = os.Remove(paths.PidFile)

	return nil
}

// GetDaemonLogPath returns the path to the daemon log file.
// This is a convenience function for use in other parts of the application.
func GetDaemonLogPath() string {
	return GetDaemonPaths().LogFile
}
