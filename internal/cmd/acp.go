package cmd

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/cloudwithax/swarmy/internal/acp"
	"github.com/spf13/cobra"
)

// DaemonManager provides an interface for daemon operations.
// This is implemented in platform-specific files using build tags.

var acpCmd = &cobra.Command{
	Use:   "acp",
	Short: "Expose Swarmy over the Agent Communication Protocol",
}

var acpServeCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start a headless ACP server",
	Long:  "Start a headless ACP server so external ACP clients can discover and orchestrate Swarmy runs.",
	RunE: func(cmd *cobra.Command, args []string) error {
		host, _ := cmd.Flags().GetString("host")
		port, _ := cmd.Flags().GetInt("port")
		autoApprove, _ := cmd.Flags().GetBool("auto-approve")
		daemon, _ := cmd.Flags().GetBool("daemon")

		if daemon {
			return startDaemon(host, port, autoApprove)
		}

		ctx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
		defer stop()

		app, err := setupApp(cmd)
		if err != nil {
			return err
		}
		defer app.Shutdown()

		if !app.Config().IsConfigured() {
			return fmt.Errorf("no providers configured - please run 'swarmy' to set up a provider interactively")
		}

		handler := acp.NewServer(
			app.AgentCoordinator,
			app.Sessions,
			app.Messages,
			app.Permissions,
			acp.Options{AutoApproveSessions: autoApprove},
		).Handler()

		server := &http.Server{
			Addr:              fmt.Sprintf("%s:%d", host, port),
			Handler:           handler,
			ReadHeaderTimeout: 10 * time.Second,
		}

		errCh := make(chan error, 1)
		go func() {
			slog.Info("Starting ACP server", "addr", server.Addr)
			if serveErr := server.ListenAndServe(); serveErr != nil && !errors.Is(serveErr, http.ErrServerClosed) {
				errCh <- serveErr
				return
			}
			errCh <- nil
		}()

		select {
		case <-ctx.Done():
			shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			return server.Shutdown(shutdownCtx)
		case serveErr := <-errCh:
			return serveErr
		}
	},
}

var acpStopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Stop the running ACP daemon",
	Long:  "Stop the ACP server daemon that was started with 'swarmy acp serve --daemon'.",
	RunE: func(cmd *cobra.Command, args []string) error {
		if !IsDaemonRunning() {
			return fmt.Errorf("no daemon is currently running")
		}

		pid := ReadPidFile()
		if err := StopDaemon(); err != nil {
			return fmt.Errorf("failed to stop daemon: %w", err)
		}

		fmt.Printf("ACP daemon stopped (PID: %d)\n", pid)
		return nil
	},
}

var acpStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Check the status of the ACP daemon",
	Long:  "Check if the ACP daemon is running and display its PID.",
	RunE: func(cmd *cobra.Command, args []string) error {
		if IsDaemonRunning() {
			pid := ReadPidFile()
			fmt.Printf("ACP daemon is running (PID: %d)\n", pid)
			fmt.Printf("Log file: %s\n", GetDaemonLogPath())
		} else {
			fmt.Println("ACP daemon is not running")
		}
		return nil
	},
}

func init() {
	acpServeCmd.Flags().String("host", "127.0.0.1", "Host interface to bind the ACP server to")
	acpServeCmd.Flags().Int("port", 8000, "Port to bind the ACP server to")
	acpServeCmd.Flags().Bool("auto-approve", true, "Automatically approve tool permissions for ACP-created runs")
	acpServeCmd.Flags().Bool("daemon", false, "Run the server in the background as a daemon")
	acpCmd.AddCommand(acpServeCmd)
	acpCmd.AddCommand(acpStopCmd)
	acpCmd.AddCommand(acpStatusCmd)
	rootCmd.AddCommand(acpCmd)
}

// startDaemon starts the ACP server in the background as a daemon process.
// This function is implemented in platform-specific files (acp_daemon_unix.go
// and acp_daemon_windows.go) using build tags.
func startDaemon(host string, port int, autoApprove bool) error {
	pid, err := StartDaemon(host, port, autoApprove)
	if err != nil {
		return err
	}
	fmt.Printf("ACP server started as daemon (PID: %d)\n", pid)
	return nil
}
