package update

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"runtime"
	"strings"
	"time"
)

const (
	resolverURLFormat = "https://github.com/cloudwithax/swarmy/releases/download/%s/resolver.json"
	resolverCacheTime = 5 * time.Minute
)

// PackageInfo contains information about a specific package.
type PackageInfo struct {
	URL      string `json:"url"`
	Format   string `json:"format"`
	Checksum string `json:"checksum,omitempty"`
}

// PlatformResolver maps package types to their download info for a platform.
type PlatformResolver struct {
	Archive *PackageInfo `json:"archive,omitempty"`
	Deb     *PackageInfo `json:"deb,omitempty"`
	RPM     *PackageInfo `json:"rpm,omitempty"`
	APK     *PackageInfo `json:"apk,omitempty"`
	ArchPkg *PackageInfo `json:"arch_pkg,omitempty"`
}

// Resolver is the root structure of the resolver JSON.
type Resolver struct {
	Version   string                       `json:"version"`
	Release   string                       `json:"release"`
	Platforms map[string]*PlatformResolver `json:"platforms"`
}

// ResolverClient fetches and parses resolver JSON files.
type ResolverClient struct {
	client      *http.Client
	cache       *Resolver
	cacheTime   time.Time
	cacheExpiry time.Duration
}

// NewResolverClient creates a new resolver client.
func NewResolverClient() *ResolverClient {
	return &ResolverClient{
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
		cacheExpiry: resolverCacheTime,
	}
}

// FetchResolver fetches the resolver JSON for a given version.
func (rc *ResolverClient) FetchResolver(ctx context.Context, version string) (*Resolver, error) {
	// Check cache first.
	if rc.cache != nil && time.Since(rc.cacheTime) < rc.cacheExpiry {
		return rc.cache, nil
	}

	url := fmt.Sprintf(resolverURLFormat, version)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Accept", "application/json")

	resp, err := rc.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch resolver: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("resolver returned status %d: %s", resp.StatusCode, string(body))
	}

	var resolver Resolver
	if err := json.NewDecoder(resp.Body).Decode(&resolver); err != nil {
		return nil, fmt.Errorf("failed to decode resolver: %w", err)
	}

	// Update cache.
	rc.cache = &resolver
	rc.cacheTime = time.Now()

	return &resolver, nil
}

// GetPackageForPlatform returns the best package for the current platform.
// It considers the detected distro on Linux to choose the appropriate package format.
func (r *Resolver) GetPackageForPlatform() (*PackageInfo, error) {
	return r.GetPackageFor(runtime.GOOS, runtime.GOARCH)
}

// GetPackageFor returns the best package for a specific platform.
func (r *Resolver) GetPackageFor(goos, goarch string) (*PackageInfo, error) {
	platformKey := buildPlatformKey(goos, goarch)

	platformResolver, ok := r.Platforms[platformKey]
	if !ok {
		return nil, fmt.Errorf("no resolver found for platform %s", platformKey)
	}

	// On Linux, try to use distro-specific package.
	if goos == "linux" {
		distro := DetectDistro()
		pkg := r.getDistroPackage(platformResolver, distro)
		if pkg != nil {
			return pkg, nil
		}
	}

	// Fall back to archive.
	if platformResolver.Archive != nil {
		return platformResolver.Archive, nil
	}

	return nil, fmt.Errorf("no package available for platform %s", platformKey)
}

// getDistroPackage returns the appropriate package for a distro.
func (r *Resolver) getDistroPackage(pr *PlatformResolver, distro DistroInfo) *PackageInfo {
	switch distro.Type {
	case DistroDebian:
		if pr.Deb != nil {
			return pr.Deb
		}
	case DistroRedHat, DistroSUSE:
		if pr.RPM != nil {
			return pr.RPM
		}
	case DistroAlpine:
		if pr.APK != nil {
			return pr.APK
		}
	case DistroArch:
		if pr.ArchPkg != nil {
			return pr.ArchPkg
		}
	}

	return nil
}

// ListAvailablePackages returns all available packages for the current platform.
func (r *Resolver) ListAvailablePackages() ([]PackageInfo, error) {
	return r.ListPackagesFor(runtime.GOOS, runtime.GOARCH)
}

// ListPackagesFor returns all available packages for a specific platform.
func (r *Resolver) ListPackagesFor(goos, goarch string) ([]PackageInfo, error) {
	platformKey := buildPlatformKey(goos, goarch)

	platformResolver, ok := r.Platforms[platformKey]
	if !ok {
		return nil, fmt.Errorf("no resolver found for platform %s", platformKey)
	}

	var packages []PackageInfo

	if platformResolver.Archive != nil {
		packages = append(packages, *platformResolver.Archive)
	}
	if platformResolver.Deb != nil {
		packages = append(packages, *platformResolver.Deb)
	}
	if platformResolver.RPM != nil {
		packages = append(packages, *platformResolver.RPM)
	}
	if platformResolver.APK != nil {
		packages = append(packages, *platformResolver.APK)
	}
	if platformResolver.ArchPkg != nil {
		packages = append(packages, *platformResolver.ArchPkg)
	}

	return packages, nil
}

// buildPlatformKey creates a platform key from GOOS and GOARCH.
func buildPlatformKey(goos, goarch string) string {
	// Normalize architecture names.
	if goarch == "amd64" {
		goarch = "x86_64"
	} else if goarch == "386" {
		goarch = "i386"
	}

	// Normalize OS names.
	goos = strings.Title(goos)

	return fmt.Sprintf("%s/%s", goos, goarch)
}

// HasPackageFormat checks if a specific package format is available.
func (r *Resolver) HasPackageFormat(goos, goarch, format string) bool {
	packages, err := r.ListPackagesFor(goos, goarch)
	if err != nil {
		return false
	}

	for _, pkg := range packages {
		if pkg.Format == format {
			return true
		}
	}

	return false
}

// GetPackageByFormat returns a specific package format if available.
func (r *Resolver) GetPackageByFormat(goos, goarch, format string) (*PackageInfo, error) {
	platformKey := buildPlatformKey(goos, goarch)

	platformResolver, ok := r.Platforms[platformKey]
	if !ok {
		return nil, fmt.Errorf("no resolver found for platform %s", platformKey)
	}

	switch format {
	case "tar.gz", "zip", "archive":
		if platformResolver.Archive != nil {
			return platformResolver.Archive, nil
		}
	case "deb":
		if platformResolver.Deb != nil {
			return platformResolver.Deb, nil
		}
	case "rpm":
		if platformResolver.RPM != nil {
			return platformResolver.RPM, nil
		}
	case "apk":
		if platformResolver.APK != nil {
			return platformResolver.APK, nil
		}
	case "arch_pkg", "pkg.tar.zst":
		if platformResolver.ArchPkg != nil {
			return platformResolver.ArchPkg, nil
		}
	}

	return nil, fmt.Errorf("format %s not available for platform %s", format, platformKey)
}
