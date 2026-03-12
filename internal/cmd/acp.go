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

func init() {
	acpServeCmd.Flags().String("host", "127.0.0.1", "Host interface to bind the ACP server to")
	acpServeCmd.Flags().Int("port", 8000, "Port to bind the ACP server to")
	acpServeCmd.Flags().Bool("auto-approve", true, "Automatically approve tool permissions for ACP-created runs")
	acpCmd.AddCommand(acpServeCmd)
	rootCmd.AddCommand(acpCmd)
}
