// generate_resolver.go generates a resolver.json file for release updates.
// It maps platform/arch combinations to their corresponding package downloads,
// including distro-specific packages for Linux.
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
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

func main() {
	if len(os.Args) < 3 {
		fmt.Fprintf(os.Stderr, "Usage: %s <version> <release_url>\n", os.Args[0])
		os.Exit(1)
	}

	version := os.Args[1]
	releaseURL := os.Args[2]

	resolver := &Resolver{
		Version:   version,
		Release:   releaseURL,
		Platforms: make(map[string]*PlatformResolver),
	}

	// Define all platforms we build for.
	platforms := []struct {
		goos   string
		goarch string
		arm    string
	}{
		{"linux", "amd64", ""},
		{"linux", "arm64", ""},
		{"linux", "386", ""},
		{"darwin", "amd64", ""},
		{"darwin", "arm64", ""},
		{"windows", "amd64", ""},
		{"windows", "arm64", ""},
		{"windows", "386", ""},
		{"freebsd", "amd64", ""},
		{"freebsd", "arm64", ""},
	}

	for _, p := range platforms {
		key := platformKey(p.goos, p.goarch, p.arm)
		resolver.Platforms[key] = buildPlatformResolver(version, p.goos, p.goarch, p.arm)
	}

	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(resolver); err != nil {
		fmt.Fprintf(os.Stderr, "Error encoding JSON: %v\n", err)
		os.Exit(1)
	}
}

func platformKey(goos, goarch, arm string) string {
	key := fmt.Sprintf("%s/%s", goos, goarch)
	if arm != "" {
		key = fmt.Sprintf("%s/v%s", key, arm)
	}
	return key
}

func buildPlatformResolver(version, goos, goarch, arm string) *PlatformResolver {
	pr := &PlatformResolver{}

	// Archive package (always available).
	archiveExt := "tar.gz"
	if goos == "windows" {
		archiveExt = "zip"
	}

	archLabel := goarch
	if goarch == "amd64" {
		archLabel = "x86_64"
	}

	archiveName := fmt.Sprintf("swarmy_%s_%s_%s.%s",
		version,
		strings.Title(goos),
		archLabel,
		archiveExt,
	)

	pr.Archive = &PackageInfo{
		URL:    artifactURL(version, archiveName),
		Format: archiveExt,
	}

	// Linux distro-specific packages.
	if goos == "linux" {
		// Debian/Ubuntu (.deb).
		debName := fmt.Sprintf("swarmy_%s_%s.deb", version, archLabel)
		pr.Deb = &PackageInfo{
			URL:    artifactURL(version, debName),
			Format: "deb",
		}

		// RedHat/Fedora (.rpm).
		rpmArch := archLabel
		if goarch == "amd64" {
			rpmArch = "x86_64"
		}
		rpmName := fmt.Sprintf("swarmy-%s-1.%s.rpm", version, rpmArch)
		pr.RPM = &PackageInfo{
			URL:    artifactURL(version, rpmName),
			Format: "rpm",
		}

		// Alpine (.apk).
		apkArch := archLabel
		if goarch == "amd64" {
			apkArch = "x86_64"
		}
		apkName := fmt.Sprintf("swarmy_%s_%s.apk", version, apkArch)
		pr.APK = &PackageInfo{
			URL:    artifactURL(version, apkName),
			Format: "apk",
		}

		// Arch Linux (pacman).
		archPkgName := fmt.Sprintf("swarmy-%s-1-%s.pkg.tar.zst", version, archLabel)
		pr.ArchPkg = &PackageInfo{
			URL:    artifactURL(version, archPkgName),
			Format: "pkg.tar.zst",
		}
	}

	return pr
}

func artifactURL(version, filename string) string {
	return fmt.Sprintf("https://github.com/cloudwithax/swarmy/releases/download/%s/%s",
		version, filename)
}
