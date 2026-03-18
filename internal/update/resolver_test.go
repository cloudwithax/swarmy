package update

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestBuildPlatformKey(t *testing.T) {
	tests := []struct {
		goos     string
		goarch   string
		expected string
	}{
		{"linux", "amd64", "Linux/x86_64"},
		{"linux", "arm64", "Linux/arm64"},
		{"linux", "386", "Linux/i386"},
		{"darwin", "amd64", "Darwin/x86_64"},
		{"darwin", "arm64", "Darwin/arm64"},
		{"windows", "amd64", "Windows/x86_64"},
		{"freebsd", "amd64", "Freebsd/x86_64"},
	}

	for _, tt := range tests {
		t.Run(tt.goos+"_"+tt.goarch, func(t *testing.T) {
			result := buildPlatformKey(tt.goos, tt.goarch)
			require.Equal(t, tt.expected, result)
		})
	}
}

func TestResolverGetPackageFor(t *testing.T) {
	resolver := &Resolver{
		Version: "v1.0.0",
		Platforms: map[string]*PlatformResolver{
			"Linux/x86_64": {
				Archive: &PackageInfo{URL: "https://example.com/archive.tar.gz", Format: "tar.gz"},
				Deb:     &PackageInfo{URL: "https://example.com/package.deb", Format: "deb"},
				RPM:     &PackageInfo{URL: "https://example.com/package.rpm", Format: "rpm"},
				APK:     &PackageInfo{URL: "https://example.com/package.apk", Format: "apk"},
				ArchPkg: &PackageInfo{URL: "https://example.com/package.pkg.tar.zst", Format: "pkg.tar.zst"},
			},
			"Darwin/arm64": {
				Archive: &PackageInfo{URL: "https://example.com/darwin.tar.gz", Format: "tar.gz"},
			},
		},
	}

	t.Run("linux with any package", func(t *testing.T) {
		pkg, err := resolver.GetPackageFor("linux", "amd64")
		require.NoError(t, err)
		require.NotNil(t, pkg)
		// Should return one of the available formats.
		validFormats := map[string]bool{"tar.gz": true, "deb": true, "rpm": true, "apk": true, "pkg.tar.zst": true}
		require.True(t, validFormats[pkg.Format], "unexpected format: %s", pkg.Format)
	})

	t.Run("darwin", func(t *testing.T) {
		pkg, err := resolver.GetPackageFor("darwin", "arm64")
		require.NoError(t, err)
		require.NotNil(t, pkg)
		require.Equal(t, "tar.gz", pkg.Format)
	})

	t.Run("unsupported platform", func(t *testing.T) {
		pkg, err := resolver.GetPackageFor("solaris", "sparc")
		require.Error(t, err)
		require.Nil(t, pkg)
	})
}

func TestResolverGetPackageByFormat(t *testing.T) {
	resolver := &Resolver{
		Platforms: map[string]*PlatformResolver{
			"Linux/x86_64": {
				Archive: &PackageInfo{URL: "https://example.com/archive.tar.gz", Format: "tar.gz"},
				Deb:     &PackageInfo{URL: "https://example.com/package.deb", Format: "deb"},
				RPM:     &PackageInfo{URL: "https://example.com/package.rpm", Format: "rpm"},
			},
		},
	}

	tests := []struct {
		format   string
		expected string
		wantErr  bool
	}{
		{"deb", "deb", false},
		{"rpm", "rpm", false},
		{"tar.gz", "tar.gz", false},
		{"apk", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.format, func(t *testing.T) {
			pkg, err := resolver.GetPackageByFormat("linux", "amd64", tt.format)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tt.expected, pkg.Format)
		})
	}
}

func TestResolverListPackagesFor(t *testing.T) {
	resolver := &Resolver{
		Platforms: map[string]*PlatformResolver{
			"Linux/x86_64": {
				Archive: &PackageInfo{Format: "tar.gz"},
				Deb:     &PackageInfo{Format: "deb"},
				RPM:     &PackageInfo{Format: "rpm"},
			},
		},
	}

	packages, err := resolver.ListPackagesFor("linux", "amd64")
	require.NoError(t, err)
	require.Len(t, packages, 3)

	formats := make(map[string]bool)
	for _, pkg := range packages {
		formats[pkg.Format] = true
	}

	require.True(t, formats["tar.gz"])
	require.True(t, formats["deb"])
	require.True(t, formats["rpm"])
}

func TestResolverHasPackageFormat(t *testing.T) {
	resolver := &Resolver{
		Platforms: map[string]*PlatformResolver{
			"Linux/x86_64": {
				Archive: &PackageInfo{Format: "tar.gz"},
				Deb:     &PackageInfo{Format: "deb"},
			},
		},
	}

	require.True(t, resolver.HasPackageFormat("linux", "amd64", "deb"))
	require.True(t, resolver.HasPackageFormat("linux", "amd64", "tar.gz"))
	require.False(t, resolver.HasPackageFormat("linux", "amd64", "rpm"))
	require.False(t, resolver.HasPackageFormat("solaris", "sparc", "deb"))
}

func TestResolverJSONRoundTrip(t *testing.T) {
	original := &Resolver{
		Version: "v1.0.0",
		Release: "https://github.com/cloudwithax/swarmy/releases/tag/v1.0.0",
		Platforms: map[string]*PlatformResolver{
			"Linux/x86_64": {
				Archive: &PackageInfo{
					URL:    "https://example.com/archive.tar.gz",
					Format: "tar.gz",
				},
				Deb: &PackageInfo{
					URL:    "https://example.com/package.deb",
					Format: "deb",
				},
			},
		},
	}

	// Encode to JSON.
	data, err := json.Marshal(original)
	require.NoError(t, err)

	// Decode from JSON.
	var decoded Resolver
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	// Verify.
	require.Equal(t, original.Version, decoded.Version)
	require.Equal(t, original.Release, decoded.Release)
	require.Len(t, decoded.Platforms, 1)

	linuxPkg := decoded.Platforms["Linux/x86_64"]
	require.NotNil(t, linuxPkg)
	require.NotNil(t, linuxPkg.Archive)
	require.NotNil(t, linuxPkg.Deb)
	require.Equal(t, "tar.gz", linuxPkg.Archive.Format)
	require.Equal(t, "deb", linuxPkg.Deb.Format)
}
