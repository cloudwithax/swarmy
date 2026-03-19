package cmd

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"runtime"
	"time"

	"charm.land/lipgloss/v2"
	"github.com/cloudwithax/swarmy/internal/update"
	"github.com/cloudwithax/swarmy/internal/version"
	"github.com/spf13/cobra"
)

var updateCmd = &cobra.Command{
	Use:   "update",
	Short: "Update swarmy to the latest version",
	Long: `Update swarmy to the latest nightly release from GitHub.

This command checks for the latest nightly release, downloads the appropriate
binary for your platform, and installs it atomically with rollback support.`,
	Example: `
# Check and update to the latest nightly release
swarmy update

# Force reinstall even if already up to date
swarmy update --force

# Preview what would be updated without making changes
swarmy update --dry-run

# Update to a stable release instead of nightly
swarmy update --nightly=false
`,
	RunE: runUpdate,
}

func init() {
	updateCmd.Flags().Bool("nightly", true, "Check for nightly releases (pre-releases)")
	updateCmd.Flags().Bool("force", false, "Force reinstall even if already up to date")
	updateCmd.Flags().Bool("dry-run", false, "Show what would be updated without making changes")
}

func runUpdate(cmd *cobra.Command, args []string) error {
	nightly, _ := cmd.Flags().GetBool("nightly")
	force, _ := cmd.Flags().GetBool("force")
	dryRun, _ := cmd.Flags().GetBool("dry-run")
	useResolver := true // Default to using resolver for private repo support

	// Cancel on SIGINT or SIGTERM.
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, os.Kill)
	defer cancel()

	// Print header.
	fmt.Println()
	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#FF5FD7"))
	fmt.Println(titleStyle.Render("🐝 Swarmy Update Checker"))
	fmt.Println()

	// Show current version and platform info.
	fmt.Printf("Current version: %s\n", version.Version)
	fmt.Printf("Platform: %s/%s\n", runtime.GOOS, runtime.GOARCH)

	// Show distro info on Linux.
	if runtime.GOOS == "linux" {
		distro := update.DetectDistro()
		if distro.Type != update.DistroUnknown {
			fmt.Printf("Distribution: %s\n", distro)
			fmt.Printf("Preferred package: %s\n", update.GetPreferredPackageFormat())
		}
	}
	fmt.Println()

	// Check for updates.
	var info update.Info
	var pkg *update.PackageInfo
	var err error

	if nightly {
		fmt.Println("Checking for latest nightly release...")
		if useResolver {
			// Use resolver-aware check which works for private repos
			info, pkg, err = update.CheckUpdateWithResolver(ctx, version.Version)
			if err != nil {
				// Fall back to traditional check if resolver fails
				fmt.Println("Resolver check failed, falling back to GitHub API...")
				info, err = update.CheckNightlyInfo(ctx, version.Version)
				pkg = nil
			}
		} else {
			info, err = update.CheckNightlyInfo(ctx, version.Version)
		}
	} else {
		fmt.Println("Checking for latest stable release...")
		info, err = update.Check(ctx, version.Version, update.Default)
	}

	if err != nil {
		return fmt.Errorf("failed to check for updates: %w", err)
	}

	// Display version information.
	fmt.Printf("Latest version: %s\n", info.Latest)
	if info.URL != "" {
		fmt.Printf("Release URL: %s\n", info.URL)
	}
	fmt.Println()

	// Check if update is needed.
	if !force && !info.Available() {
		fmt.Println(lipgloss.NewStyle().Foreground(lipgloss.Color("#73F59F")).Render("✓ You're already running the latest version!"))
		return nil
	}

	if force {
		fmt.Println(lipgloss.NewStyle().Foreground(lipgloss.Color("#F5C973")).Render("⚠ Force flag set: reinstalling even if up to date"))
		fmt.Println()
	}

	if dryRun {
		fmt.Println(lipgloss.NewStyle().Foreground(lipgloss.Color("#73A2F5")).Render("→ Dry run mode: no changes will be made"))
		fmt.Println()
		fmt.Println("Would download and install:")
		if pkg != nil {
			fmt.Printf("  Package: %s\n", pkg.URL)
		} else {
			fmt.Printf("  Asset: %s\n", update.GetAssetName())
		}
		binaryPath, _ := update.GetBinaryPath()
		fmt.Printf("  Target: %s\n", binaryPath)
		return nil
	}

	// Find the correct asset for this platform.
	if pkg != nil {
		fmt.Printf("Target package: %s\n", pkg.URL)
	} else {
		assetName := update.GetAssetName()
		fmt.Printf("Target asset: %s\n", assetName)
	}
	fmt.Println()

	// Download the update.
	fmt.Println("Downloading update...")
	startTime := time.Now()

	progressFn := func(downloaded, total int64) {
		if total > 0 {
			percent := float64(downloaded) * 100 / float64(total)
			fmt.Printf("\r  Progress: %.1f%% (%s / %s)",
				percent,
				formatBytes(downloaded),
				formatBytes(total),
			)
		} else {
			fmt.Printf("\r  Downloaded: %s", formatBytes(downloaded))
		}
	}

	var updatePath string
	if pkg != nil {
		// Use resolver package download
		updatePath, err = update.DownloadPackageToTemp(ctx, pkg, progressFn)
	} else {
		// Fall back to traditional asset download
		updatePath, err = update.DownloadToTemp(ctx, progressFn)
	}
	if err != nil {
		fmt.Println() // Clear the progress line.
		return fmt.Errorf("failed to download update: %w", err)
	}

	fmt.Println() // Clear the progress line.
	downloadDuration := time.Since(startTime)
	fmt.Printf("Downloaded in %s\n", downloadDuration.Round(time.Millisecond))
	fmt.Println()

	// Install the update.
	fmt.Println("Installing update...")

	platform := update.CurrentPlatform()
	backupPath, err := update.Install(updatePath, platform)
	if err != nil {
		// Attempt to rollback on failure.
		if backupPath != "" {
			fmt.Println("Installation failed, attempting to restore backup...")
			if rbErr := update.Rollback(backupPath); rbErr != nil {
				return fmt.Errorf("installation failed and rollback failed: %w (rollback error: %v)", err, rbErr)
			}
			return fmt.Errorf("installation failed but rollback succeeded: %w", err)
		}
		return fmt.Errorf("failed to install update: %w", err)
	}

	// Success!
	fmt.Println()
	successStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#73F59F"))
	fmt.Println(successStyle.Render("✓ Successfully updated swarmy!"))
	fmt.Printf("\nNew version: %s\n", info.Latest)
	fmt.Println("\nPlease restart swarmy to use the new version.")

	return nil
}

// formatBytes converts bytes to human-readable format.
func formatBytes(b int64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(b)/float64(div), "KMGTPE"[exp])
}

// progressReader wraps an io.Reader to track progress.
type progressReader struct {
	reader     io.Reader
	total      int64
	downloaded int64
	onProgress func(downloaded, total int64)
}

func newProgressReader(reader io.Reader, total int64, onProgress func(downloaded, total int64)) *progressReader {
	return &progressReader{
		reader:     reader,
		total:      total,
		onProgress: onProgress,
	}
}

func (pr *progressReader) Read(p []byte) (n int, err error) {
	n, err = pr.reader.Read(p)
	pr.downloaded += int64(n)
	if pr.onProgress != nil {
		pr.onProgress(pr.downloaded, pr.total)
	}
	return n, err
}
