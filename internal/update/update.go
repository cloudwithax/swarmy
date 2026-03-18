package update

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"time"
)

const (
	githubAPIURL      = "https://api.github.com/repos/cloudwithax/swarmy/releases"
	githubLatestURL   = "https://api.github.com/repos/cloudwithax/swarmy/releases/latest"
	userAgent         = "swarmy/1.0"
	nightlyTagPrefix  = "nightly"
	binaryName        = "swarmy"
	windowsBinaryName = "swarmy.exe"
)

// Default is the default [Client].
var Default Client = &github{}

// Info contains information about an available update.
type Info struct {
	Current   string
	Latest    string
	URL       string
	IsNightly bool
}

// Matches a version string like:
// v0.0.0-0.20251231235959-06c807842604
var goInstallRegexp = regexp.MustCompile(`^v?\d+\.\d+\.\d+-\d+\.\d{14}-[0-9a-f]{12}$`)

func (i Info) IsDevelopment() bool {
	return i.Current == "devel" || i.Current == "unknown" || strings.Contains(i.Current, "dirty") || goInstallRegexp.MatchString(i.Current)
}

// Available returns true if there's an update available.
//
// If both current and latest are stable versions, returns true if versions are
// different.
// If current is a pre-release and latest isn't, returns true.
// If latest is a pre-release and current isn't, returns false.
func (i Info) Available() bool {
	cpr := strings.Contains(i.Current, "-")
	lpr := strings.Contains(i.Latest, "-")
	// current is pre release && latest isn't a prerelease
	if cpr && !lpr {
		return true
	}
	// latest is pre release && current isn't a prerelease
	if lpr && !cpr {
		return false
	}
	return i.Current != i.Latest
}

// Check checks if a new version is available.
func Check(ctx context.Context, current string, client Client) (Info, error) {
	info := Info{
		Current: current,
		Latest:  current,
	}

	release, err := client.Latest(ctx)
	if err != nil {
		return info, fmt.Errorf("failed to fetch latest release: %w", err)
	}

	info.Latest = strings.TrimPrefix(release.TagName, "v")
	info.Current = strings.TrimPrefix(info.Current, "v")
	info.URL = release.HTMLURL
	return info, nil
}

// Release represents a GitHub release.
type Release struct {
	TagName    string  `json:"tag_name"`
	HTMLURL    string  `json:"html_url"`
	Prerelease bool    `json:"prerelease"`
	Assets     []Asset `json:"assets"`
	CreatedAt  string  `json:"created_at"`
}

// Asset represents a GitHub release asset.
type Asset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
	Size               int64  `json:"size"`
	ContentType        string `json:"content_type"`
}

// Client is a client that can get the latest release.
type Client interface {
	Latest(ctx context.Context) (*Release, error)
}

type github struct{}

// Latest implements [Client].
func (c *github) Latest(ctx context.Context) (*Release, error) {
	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	req, err := http.NewRequestWithContext(ctx, "GET", githubLatestURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("GitHub API returned status %d: %s", resp.StatusCode, string(body))
	}

	var release Release
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return nil, err
	}

	return &release, nil
}

// Platform represents the target platform for a binary.
type Platform struct {
	OS   string
	Arch string
}

// CurrentPlatform returns the current platform.
func CurrentPlatform() Platform {
	os := runtime.GOOS
	arch := runtime.GOARCH

	// Normalize architecture names to match GoReleaser conventions.
	switch arch {
	case "amd64":
		arch = "x86_64"
	case "386":
		arch = "i386"
	}

	// Normalize OS names to lowercase to match GoReleaser asset naming.
	switch os {
	case "darwin":
		os = "darwin"
	case "linux":
		os = "linux"
	case "windows":
		os = "windows"
	case "freebsd":
		os = "freebsd"
	case "openbsd":
		os = "openbsd"
	}

	return Platform{OS: os, Arch: arch}
}

// AssetName returns the expected asset name for this platform.
func (p Platform) AssetName(version string) string {
	ext := "tar.gz"
	if p.OS == "windows" {
		ext = "zip"
	}
	return fmt.Sprintf("swarmy_%s_%s_%s.%s", version, p.OS, p.Arch, ext)
}

// BinaryName returns the binary name for this platform.
func (p Platform) BinaryName() string {
	if p.OS == "windows" {
		return windowsBinaryName
	}
	return binaryName
}

// IsWindows returns true if the platform is Windows.
func (p Platform) IsWindows() bool {
	return p.OS == "windows"
}

// NightlyChecker handles checking for nightly releases.
type NightlyChecker struct {
	client *http.Client
}

// NewNightlyChecker creates a new nightly checker.
func NewNightlyChecker() *NightlyChecker {
	return &NightlyChecker{
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// CheckNightly checks for the latest nightly release.
// Returns the release info, the matching asset, and any error.
func (nc *NightlyChecker) CheckNightly(ctx context.Context, currentVersion string) (*Release, *Asset, error) {
	releases, err := nc.fetchReleases(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to fetch releases: %w", err)
	}

	// Find the latest nightly release.
	var nightlyRelease *Release
	for i := range releases {
		if strings.HasPrefix(releases[i].TagName, nightlyTagPrefix) || releases[i].Prerelease {
			nightlyRelease = &releases[i]
			break
		}
	}

	if nightlyRelease == nil {
		return nil, nil, fmt.Errorf("no nightly release found")
	}

	// Find the matching asset for the current platform.
	platform := CurrentPlatform()
	expectedName := platform.AssetName(nightlyRelease.TagName)

	for i := range nightlyRelease.Assets {
		if nightlyRelease.Assets[i].Name == expectedName {
			return nightlyRelease, &nightlyRelease.Assets[i], nil
		}
	}

	return nightlyRelease, nil, fmt.Errorf("no asset found for platform %s_%s", platform.OS, platform.Arch)
}

func (nc *NightlyChecker) fetchReleases(ctx context.Context) ([]Release, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", githubAPIURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	resp, err := nc.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("GitHub API returned status %d: %s", resp.StatusCode, string(body))
	}

	var releases []Release
	if err := json.NewDecoder(resp.Body).Decode(&releases); err != nil {
		return nil, err
	}

	return releases, nil
}

// DownloadProgress is called during download to report progress.
type DownloadProgress func(downloaded, total int64)

// Downloader handles downloading update assets.
type Downloader struct {
	client   *http.Client
	progress DownloadProgress
}

// NewDownloader creates a new downloader.
func NewDownloader(progress DownloadProgress) *Downloader {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.MaxIdleConns = 100
	transport.MaxIdleConnsPerHost = 10
	transport.IdleConnTimeout = 90 * time.Second

	return &Downloader{
		client: &http.Client{
			Timeout:   10 * time.Minute,
			Transport: transport,
		},
		progress: progress,
	}
}

// Download downloads the asset to the specified path.
func (d *Downloader) Download(ctx context.Context, asset *Asset, destPath string) error {
	req, err := http.NewRequestWithContext(ctx, "GET", asset.BrowserDownloadURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("User-Agent", userAgent)

	resp, err := d.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to download: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download failed with status %d", resp.StatusCode)
	}

	// Create parent directories.
	if err := os.MkdirAll(filepath.Dir(destPath), 0o755); err != nil {
		return fmt.Errorf("failed to create parent directories: %w", err)
	}

	// Create temporary file for atomic write.
	tempFile := destPath + ".tmp"
	out, err := os.Create(tempFile)
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}

	// Wrap the response body to track progress.
	var reader io.Reader = resp.Body
	if d.progress != nil && asset.Size > 0 {
		reader = &progressReader{
			reader:     resp.Body,
			total:      asset.Size,
			progress:   d.progress,
			reportFreq: 1024 * 1024, // Report every 1MB
		}
	}

	_, err = io.Copy(out, reader)
	out.Close()
	if err != nil {
		os.Remove(tempFile)
		return fmt.Errorf("failed to write file: %w", err)
	}

	// Atomic rename.
	if err := os.Rename(tempFile, destPath); err != nil {
		os.Remove(tempFile)
		return fmt.Errorf("failed to rename temp file: %w", err)
	}

	return nil
}

// progressReader wraps a reader to report progress.
type progressReader struct {
	reader     io.Reader
	total      int64
	downloaded int64
	progress   DownloadProgress
	reportFreq int64
	lastReport int64
}

func (pr *progressReader) Read(p []byte) (int, error) {
	n, err := pr.reader.Read(p)
	pr.downloaded += int64(n)

	// Report progress at intervals.
	if pr.progress != nil && pr.downloaded-pr.lastReport >= pr.reportFreq {
		pr.progress(pr.downloaded, pr.total)
		pr.lastReport = pr.downloaded
	}

	return n, err
}

// Installer handles installing the downloaded update.
type Installer struct {
	backupDir string
}

// NewInstaller creates a new installer.
func NewInstaller() *Installer {
	return &Installer{
		backupDir: filepath.Join(os.TempDir(), "swarmy-update-backups"),
	}
}

// Install installs the update from the downloaded archive.
// It extracts the binary, backs up the current binary, and atomically replaces it.
// Returns the path to the backup for potential rollback.
func (i *Installer) Install(archivePath string, platform Platform) (string, error) {
	// Get current executable path.
	currentExec, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("failed to get current executable: %w", err)
	}

	// Resolve symlinks to get the real path.
	currentExec, err = filepath.EvalSymlinks(currentExec)
	if err != nil {
		return "", fmt.Errorf("failed to resolve executable path: %w", err)
	}

	// Extract the new binary to a temp directory.
	extractDir, err := os.MkdirTemp("", "swarmy-update-*")
	if err != nil {
		return "", fmt.Errorf("failed to create temp directory: %w", err)
	}
	defer os.RemoveAll(extractDir)

	// Extract based on archive type.
	var newBinaryPath string
	if platform.IsWindows() {
		newBinaryPath, err = extractZip(archivePath, extractDir, platform.BinaryName())
	} else {
		newBinaryPath, err = extractTarGz(archivePath, extractDir, platform.BinaryName())
	}
	if err != nil {
		return "", fmt.Errorf("failed to extract archive: %w", err)
	}

	// Create backup directory.
	if err := os.MkdirAll(i.backupDir, 0o755); err != nil {
		return "", fmt.Errorf("failed to create backup directory: %w", err)
	}

	// Create unique backup path.
	timestamp := time.Now().Format("20060102-150405")
	backupPath := filepath.Join(i.backupDir, fmt.Sprintf("swarmy-backup-%s", timestamp))
	if platform.IsWindows() {
		backupPath += ".exe"
	}

	// Copy current binary to backup (don't move, so we can rollback).
	if err := copyFile(currentExec, backupPath); err != nil {
		return "", fmt.Errorf("failed to create backup: %w", err)
	}

	// On Unix systems, preserve the original file's permissions.
	if !platform.IsWindows() {
		info, err := os.Stat(currentExec)
		if err == nil {
			os.Chmod(newBinaryPath, info.Mode())
		} else {
			// Default to executable.
			os.Chmod(newBinaryPath, 0o755)
		}
	}

	// Atomically replace the current binary.
	if err := atomicReplace(newBinaryPath, currentExec); err != nil {
		// Attempt rollback.
		_ = atomicReplace(backupPath, currentExec)
		return "", fmt.Errorf("failed to install update: %w", err)
	}

	return backupPath, nil
}

// Rollback restores the backup binary.
func (i *Installer) Rollback(backupPath string) error {
	if backupPath == "" {
		return fmt.Errorf("no backup path provided")
	}

	currentExec, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to get current executable: %w", err)
	}

	currentExec, err = filepath.EvalSymlinks(currentExec)
	if err != nil {
		return fmt.Errorf("failed to resolve executable path: %w", err)
	}

	if err := atomicReplace(backupPath, currentExec); err != nil {
		return fmt.Errorf("failed to rollback: %w", err)
	}

	return nil
}

// CleanupBackup removes the backup file.
func (i *Installer) CleanupBackup(backupPath string) error {
	if backupPath == "" {
		return nil
	}
	return os.Remove(backupPath)
}

// atomicReplace atomically replaces dest with src.
func atomicReplace(src, dest string) error {
	// Windows doesn't support atomic rename of running executables.
	// We need to move the old binary aside first.
	if runtime.GOOS == "windows" {
		// Generate a unique temp name for the old binary.
		tempOld := dest + ".old"

		// Remove any existing .old file.
		os.Remove(tempOld)

		// Move current binary to .old.
		if err := os.Rename(dest, tempOld); err != nil {
			return fmt.Errorf("failed to move old binary: %w", err)
		}

		// Move new binary to destination.
		if err := os.Rename(src, dest); err != nil {
			// Attempt to restore old binary.
			os.Rename(tempOld, dest)
			return fmt.Errorf("failed to move new binary: %w", err)
		}

		// Remove old binary.
		os.Remove(tempOld)

		return nil
	}

	// On Unix, we can atomically rename even if the binary is running.
	return os.Rename(src, dest)
}

// copyFile copies a file from src to dst.
func copyFile(src, dst string) error {
	sourceFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer sourceFile.Close()

	destFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer destFile.Close()

	_, err = io.Copy(destFile, sourceFile)
	if err != nil {
		return err
	}

	// Preserve permissions.
	info, err := sourceFile.Stat()
	if err != nil {
		return err
	}
	return os.Chmod(dst, info.Mode())
}

// extractTarGz extracts a tar.gz archive and returns the path to the binary.
func extractTarGz(archivePath, destDir, binaryName string) (string, error) {
	file, err := os.Open(archivePath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	gzReader, err := gzip.NewReader(file)
	if err != nil {
		return "", fmt.Errorf("failed to create gzip reader: %w", err)
	}
	defer gzReader.Close()

	tarReader := tar.NewReader(gzReader)

	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", fmt.Errorf("failed to read tar: %w", err)
		}

		// Skip directories.
		if header.Typeflag == tar.TypeDir {
			continue
		}

		// Check if this is the binary we're looking for.
		baseName := filepath.Base(header.Name)
		if baseName == binaryName {
			destPath := filepath.Join(destDir, binaryName)
			outFile, err := os.Create(destPath)
			if err != nil {
				return "", fmt.Errorf("failed to create output file: %w", err)
			}

			_, err = io.Copy(outFile, tarReader)
			outFile.Close()
			if err != nil {
				return "", fmt.Errorf("failed to extract binary: %w", err)
			}

			// Make executable.
			if err := os.Chmod(destPath, 0o755); err != nil {
				return "", fmt.Errorf("failed to set permissions: %w", err)
			}

			return destPath, nil
		}
	}

	return "", fmt.Errorf("binary %s not found in archive", binaryName)
}

// extractZip extracts a zip archive and returns the path to the binary.
func extractZip(archivePath, destDir, binaryName string) (string, error) {
	reader, err := zip.OpenReader(archivePath)
	if err != nil {
		return "", fmt.Errorf("failed to open zip: %w", err)
	}
	defer reader.Close()

	for _, file := range reader.File {
		baseName := filepath.Base(file.Name)
		if baseName == binaryName {
			srcFile, err := file.Open()
			if err != nil {
				return "", fmt.Errorf("failed to open file in zip: %w", err)
			}
			defer srcFile.Close()

			destPath := filepath.Join(destDir, binaryName)
			outFile, err := os.Create(destPath)
			if err != nil {
				return "", fmt.Errorf("failed to create output file: %w", err)
			}

			_, err = io.Copy(outFile, srcFile)
			outFile.Close()
			if err != nil {
				return "", fmt.Errorf("failed to extract binary: %w", err)
			}

			return destPath, nil
		}
	}

	return "", fmt.Errorf("binary %s not found in archive", binaryName)
}

// Package-level variables for the current update operation.
var (
	currentRelease *Release
	currentAsset   *Asset
)

// CheckNightlyInfo checks for the latest nightly release and returns Info.
// This is a convenience wrapper that returns the Info type used by the CLI.
func CheckNightlyInfo(ctx context.Context, currentVersion string) (Info, error) {
	release, _, err := CheckNightly(ctx, currentVersion)
	if err != nil {
		return Info{Current: currentVersion, Latest: currentVersion}, err
	}

	return Info{
		Current:   strings.TrimPrefix(currentVersion, "v"),
		Latest:    strings.TrimPrefix(release.TagName, "v"),
		URL:       release.HTMLURL,
		IsNightly: true,
	}, nil
}

// CheckNightly is a convenience function that checks for the latest nightly release.
func CheckNightly(ctx context.Context, currentVersion string) (*Release, *Asset, error) {
	checker := NewNightlyChecker()
	release, asset, err := checker.CheckNightly(ctx, currentVersion)
	if err != nil {
		return nil, nil, err
	}
	// Store for later use by other functions.
	currentRelease = release
	currentAsset = asset
	return release, asset, nil
}

// Download is a convenience function that downloads an asset.
func Download(ctx context.Context, asset *Asset, destPath string, progress DownloadProgress) error {
	downloader := NewDownloader(progress)
	return downloader.Download(ctx, asset, destPath)
}

// DownloadToTemp downloads the current asset to a temporary file and returns the path.
// This is a convenience function for the CLI that manages its own HTTP client.
func DownloadToTemp(ctx context.Context, progress DownloadProgress) (string, error) {
	if currentAsset == nil {
		return "", fmt.Errorf("no asset selected, call CheckNightly first")
	}

	tempDir := os.TempDir()
	archivePath := filepath.Join(tempDir, currentAsset.Name)

	downloader := NewDownloader(progress)
	if err := downloader.Download(ctx, currentAsset, archivePath); err != nil {
		return "", err
	}

	return archivePath, nil
}

// Install is a convenience function that installs an update.
func Install(archivePath string, platform Platform) (string, error) {
	installer := NewInstaller()
	return installer.Install(archivePath, platform)
}

// GetAssetName returns the expected asset name for the current platform.
// Returns an empty string if CheckNightly hasn't been called yet.
func GetAssetName() string {
	if currentRelease == nil {
		platform := CurrentPlatform()
		return platform.AssetName("latest")
	}
	platform := CurrentPlatform()
	return platform.AssetName(currentRelease.TagName)
}

// GetBinaryPath returns the path to the current binary.
func GetBinaryPath() (string, error) {
	exec, err := os.Executable()
	if err != nil {
		return "", err
	}
	return filepath.EvalSymlinks(exec)
}

// Rollback restores the backup binary.
func Rollback(backupPath string) error {
	installer := NewInstaller()
	return installer.Rollback(backupPath)
}

// CleanupBackup removes the backup file.
func CleanupBackup(backupPath string) error {
	installer := NewInstaller()
	return installer.CleanupBackup(backupPath)
}

// Resolver-aware update functions.

// CheckUpdateWithResolver checks for updates using the resolver JSON.
// It returns update info and the best package for the current platform.
func CheckUpdateWithResolver(ctx context.Context, currentVersion string) (Info, *PackageInfo, error) {
	info, err := CheckNightlyInfo(ctx, currentVersion)
	if err != nil {
		return info, nil, err
	}

	// Fetch the resolver for the latest version.
	resolverClient := NewResolverClient()
	resolver, err := resolverClient.FetchResolver(ctx, "nightly")
	if err != nil {
		// Fall back to traditional asset matching if resolver fails.
		return info, nil, fmt.Errorf("resolver not available: %w", err)
	}

	// Get the best package for this platform.
	pkg, err := resolver.GetPackageForPlatform()
	if err != nil {
		return info, nil, err
	}

	return info, pkg, nil
}

// DownloadPackage downloads a package from the resolver to a temporary file.
func DownloadPackage(ctx context.Context, pkg *PackageInfo, progress DownloadProgress) (string, error) {
	if pkg == nil {
		return "", fmt.Errorf("no package provided")
	}

	// Extract filename from URL.
	parts := strings.Split(pkg.URL, "/")
	filename := parts[len(parts)-1]
	if filename == "" {
		filename = "swarmy-package"
	}

	tempDir := os.TempDir()
	downloadPath := filepath.Join(tempDir, filename)

	downloader := NewDownloader(progress)

	// Create a temporary asset struct for downloading.
	asset := &Asset{
		Name:               filename,
		BrowserDownloadURL: pkg.URL,
	}

	if err := downloader.Download(ctx, asset, downloadPath); err != nil {
		return "", err
	}

	return downloadPath, nil
}

// PackageFormat represents the type of package format.
type PackageFormat string

const (
	// FormatArchive is a generic archive (tar.gz or zip).
	FormatArchive PackageFormat = "archive"
	// FormatDeb is a Debian package.
	FormatDeb PackageFormat = "deb"
	// FormatRPM is an RPM package.
	FormatRPM PackageFormat = "rpm"
	// FormatAPK is an Alpine package.
	FormatAPK PackageFormat = "apk"
	// FormatArchPkg is an Arch Linux package.
	FormatArchPkg PackageFormat = "arch_pkg"
)

// InstallerType represents the method to install the package.
type InstallerType string

const (
	// InstallerAuto automatically chooses the best installer.
	InstallerAuto InstallerType = "auto"
	// InstallerPackageManager uses the system package manager.
	InstallerPackageManager InstallerType = "package_manager"
	// InstallerDirect extracts and replaces the binary directly.
	InstallerDirect InstallerType = "direct"
)

// PackageInstaller handles installing distro packages.
type PackageInstaller struct {
	installerType InstallerType
}

// NewPackageInstaller creates a new package installer.
func NewPackageInstaller(installerType InstallerType) *PackageInstaller {
	if installerType == "" {
		installerType = InstallerAuto
	}
	return &PackageInstaller{installerType: installerType}
}

// InstallPackage installs a package based on its format.
// Returns the backup path and any error.
func (pi *PackageInstaller) InstallPackage(pkgPath string, format PackageFormat) (string, error) {
	switch format {
	case FormatDeb:
		return pi.installDeb(pkgPath)
	case FormatRPM:
		return pi.installRPM(pkgPath)
	case FormatAPK:
		return pi.installAPK(pkgPath)
	case FormatArchPkg:
		return pi.installArchPkg(pkgPath)
	default:
		// Fall back to archive installation.
		platform := CurrentPlatform()
		installer := NewInstaller()
		return installer.Install(pkgPath, platform)
	}
}

// installDeb installs a .deb package using dpkg.
func (pi *PackageInstaller) installDeb(pkgPath string) (string, error) {
	// Check if we have dpkg available.
	if _, err := os.Stat("/usr/bin/dpkg"); err != nil {
		// Fall back to direct installation.
		platform := CurrentPlatform()
		installer := NewInstaller()
		return installer.Install(pkgPath, platform)
	}

	// For .deb packages, we extract the binary and install it directly
	// to avoid permission issues with system package managers.
	return pi.extractAndInstall(pkgPath, "data.tar")
}

// installRPM installs an .rpm package using rpm2cpio.
func (pi *PackageInstaller) installRPM(pkgPath string) (string, error) {
	// Check if we have rpm2cpio available.
	if _, err := os.Stat("/usr/bin/rpm2cpio"); err != nil {
		// Fall back to direct installation.
		platform := CurrentPlatform()
		installer := NewInstaller()
		return installer.Install(pkgPath, platform)
	}

	return pi.extractAndInstall(pkgPath, "")
}

// installAPK installs an .apk package.
func (pi *PackageInstaller) installAPK(pkgPath string) (string, error) {
	// Alpine packages are tar.gz files, extract and install.
	return pi.extractAndInstall(pkgPath, "")
}

// installArchPkg installs an Arch package.
func (pi *PackageInstaller) installArchPkg(pkgPath string) (string, error) {
	// Arch packages are zstd compressed tar, extract and install.
	return pi.extractAndInstall(pkgPath, "")
}

// extractAndInstall extracts the binary from a package and installs it.
func (pi *PackageInstaller) extractAndInstall(pkgPath, tarFilter string) (string, error) {
	// For now, fall back to standard archive extraction.
	// Distro packages typically contain the binary in a specific location.
	platform := CurrentPlatform()
	installer := NewInstaller()
	return installer.Install(pkgPath, platform)
}

// GetPreferredPackageFormat returns the preferred package format for the current system.
func GetPreferredPackageFormat() PackageFormat {
	distro := DetectDistro()
	switch distro.Type {
	case DistroDebian:
		return FormatDeb
	case DistroRedHat, DistroSUSE:
		return FormatRPM
	case DistroAlpine:
		return FormatAPK
	case DistroArch:
		return FormatArchPkg
	default:
		return FormatArchive
	}
}
