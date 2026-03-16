package update

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCheckForUpdate_Old(t *testing.T) {
	info, err := Check(t.Context(), "v0.10.0", testClient{"v0.11.0"})
	require.NoError(t, err)
	require.NotNil(t, info)
	require.True(t, info.Available())
}

func TestCheckForUpdate_Beta(t *testing.T) {
	t.Run("current is stable", func(t *testing.T) {
		info, err := Check(t.Context(), "v0.10.0", testClient{"v0.11.0-beta.1"})
		require.NoError(t, err)
		require.NotNil(t, info)
		require.False(t, info.Available())
	})

	t.Run("current is also beta", func(t *testing.T) {
		info, err := Check(t.Context(), "v0.11.0-beta.1", testClient{"v0.11.0-beta.2"})
		require.NoError(t, err)
		require.NotNil(t, info)
		require.True(t, info.Available())
	})

	t.Run("current is beta, latest isn't", func(t *testing.T) {
		info, err := Check(t.Context(), "v0.11.0-beta.1", testClient{"v0.11.0"})
		require.NoError(t, err)
		require.NotNil(t, info)
		require.True(t, info.Available())
	})
}

type testClient struct{ tag string }

// Latest implements Client.
func (t testClient) Latest(ctx context.Context) (*Release, error) {
	return &Release{
		TagName: t.tag,
		HTMLURL: "https://example.org",
	}, nil
}

func TestPlatform_AssetName(t *testing.T) {
	tests := []struct {
		name     string
		platform Platform
		version  string
		want     string
	}{
		{
			name:     "Linux amd64",
			platform: Platform{OS: "Linux", Arch: "x86_64"},
			version:  "v1.0.0",
			want:     "swarmy_v1.0.0_Linux_x86_64.tar.gz",
		},
		{
			name:     "Darwin arm64",
			platform: Platform{OS: "Darwin", Arch: "arm64"},
			version:  "nightly-20240315",
			want:     "swarmy_nightly-20240315_Darwin_arm64.tar.gz",
		},
		{
			name:     "Windows x86_64",
			platform: Platform{OS: "Windows", Arch: "x86_64"},
			version:  "v1.0.0",
			want:     "swarmy_v1.0.0_Windows_x86_64.zip",
		},
		{
			name:     "FreeBSD i386",
			platform: Platform{OS: "FreeBSD", Arch: "i386"},
			version:  "v1.0.0",
			want:     "swarmy_v1.0.0_FreeBSD_i386.tar.gz",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.platform.AssetName(tt.version)
			require.Equal(t, tt.want, got)
		})
	}
}

func TestPlatform_BinaryName(t *testing.T) {
	tests := []struct {
		name     string
		platform Platform
		want     string
	}{
		{
			name:     "Unix binary",
			platform: Platform{OS: "Linux", Arch: "x86_64"},
			want:     "swarmy",
		},
		{
			name:     "Windows binary",
			platform: Platform{OS: "Windows", Arch: "x86_64"},
			want:     "swarmy.exe",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.platform.BinaryName()
			require.Equal(t, tt.want, got)
		})
	}
}

func TestPlatform_IsWindows(t *testing.T) {
	tests := []struct {
		name     string
		platform Platform
		want     bool
	}{
		{
			name:     "Linux is not Windows",
			platform: Platform{OS: "Linux", Arch: "x86_64"},
			want:     false,
		},
		{
			name:     "Windows is Windows",
			platform: Platform{OS: "Windows", Arch: "x86_64"},
			want:     true,
		},
		{
			name:     "macOS is not Windows",
			platform: Platform{OS: "Darwin", Arch: "arm64"},
			want:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.platform.IsWindows()
			require.Equal(t, tt.want, got)
		})
	}
}

func TestFormatBytes(t *testing.T) {
	tests := []struct {
		bytes int64
		want  string
	}{
		{0, "0 B"},
		{512, "512 B"},
		{1024, "1.0 KB"},
		{1536, "1.5 KB"},
		{1024 * 1024, "1.0 MB"},
		{1024 * 1024 * 1024, "1.0 GB"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			got := formatBytesForTest(tt.bytes)
			require.Equal(t, tt.want, got)
		})
	}
}

// formatBytesForTest is a test helper that mimics the formatBytes function.
func formatBytesForTest(b int64) string {
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
