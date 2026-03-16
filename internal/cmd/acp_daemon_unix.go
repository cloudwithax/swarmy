//go:build linux || darwin

package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"syscall"

	"github.com/cloudwithax/swarmy/internal/home"
)

// DaemonPaths holds the paths for daemon-related files.
type DaemonPaths struct {
	LogFile string
	PidFile string
}

// GetDaemonPaths returns the paths for daemon log and PID files.
// It uses the XDG_DATA_HOME environment variable if set, otherwise
// defaults to ~/.local/share/swarmy/.
func GetDaemonPaths() DaemonPaths {
	dataDir := getDataDirectory()
	return DaemonPaths{
		LogFile: filepath.Join(dataDir, "acp-daemon.log"),
		PidFile: filepath.Join(dataDir, "acp-daemon.pid"),
	}
}

// getDataDirectory returns the platform-appropriate data directory.
// It checks SWARMY_GLOBAL_DATA and XDG_DATA_HOME environment variables,
// falling back to ~/.local/share/swarmy/.
func getDataDirectory() string {
	if swarmyData := os.Getenv("SWARMY_GLOBAL_DATA"); swarmyData != "" {
		return swarmyData
	}
	if xdgDataHome := os.Getenv("XDG_DATA_HOME"); xdgDataHome != "" {
		return filepath.Join(xdgDataHome, "swarmy")
	}
	return filepath.Join(home.Dir(), ".local", "share", "swarmy")
}

// StartDaemon starts the ACP server as a background daemon process.
// It forks the current process, redirects output to a log file, creates
// a PID file, and returns the PID of the daemon process.
func StartDaemon(host string, port int, autoApprove bool) (int, error) {
	paths := GetDaemonPaths()

	// Ensure the data directory exists.
	dataDir := filepath.Dir(paths.LogFile)
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		return 0, fmt.Errorf("failed to create data directory: %w", err)
	}

	// Check if daemon is already running.
	if IsDaemonRunning() {
		return 0, fmt.Errorf("daemon is already running (PID: %d)", ReadPidFile())
	}

	// Open the log file for appending (create if doesn't exist).
	logFile, err := os.OpenFile(paths.LogFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return 0, fmt.Errorf("failed to open log file: %w", err)
	}

	// Get the current executable path.
	execPath, err := os.Executable()
	if err != nil {
		logFile.Close()
		return 0, fmt.Errorf("failed to get executable path: %w", err)
	}

	// Build the command arguments - filter out --daemon flag and add host/port/auto-approve.
	args := buildDaemonArgs(host, port, autoApprove)

	// Set up the process attributes for daemonization.
	// We use StartProcess with proper attributes to detach from the terminal.
	attr := &os.ProcAttr{
		Dir:   ".",
		Env:   append(os.Environ(), "SWARMY_DAEMON_CHILD=1"),
		Files: []*os.File{nil, logFile, logFile},
		Sys: &syscall.SysProcAttr{
			Setsid: true, // Create new session to detach from terminal.
		},
	}

	// Start the daemon process.
	process, err := os.StartProcess(execPath, append([]string{os.Args[0]}, args...), attr)
	logFile.Close()

	if err != nil {
		return 0, fmt.Errorf("failed to start daemon process: %w", err)
	}

	// Write the PID file.
	if err := writePidFile(paths.PidFile, process.Pid); err != nil {
		// Attempt to kill the process if we can't write the PID file.
		_ = process.Kill()
		return 0, fmt.Errorf("failed to write PID file: %w", err)
	}

	return process.Pid, nil
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

// writePidFile writes the PID to the PID file with appropriate permissions.
func writePidFile(pidFile string, pid int) error {
	// Write the PID atomically using a temp file and rename.
	tmpFile := pidFile + ".tmp"
	pidStr := strconv.Itoa(pid) + "\n"

	if err := os.WriteFile(tmpFile, []byte(pidStr), 0o644); err != nil {
		return err
	}

	return os.Rename(tmpFile, pidFile)
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

	// Check if process exists by sending signal 0.
	process, err := os.FindProcess(pid)
	if err != nil {
		return false
	}

	err = process.Signal(syscall.Signal(0))
	return err == nil
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

	// Send SIGTERM first for graceful shutdown.
	if err := process.Signal(syscall.SIGTERM); err != nil {
		// Process may have already exited, try SIGKILL.
		_ = process.Kill()
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

// IsDaemonChild returns true if this process is the daemon child process.
func IsDaemonChild() bool {
	return os.Getenv("SWARMY_DAEMON_CHILD") == "1"
}
