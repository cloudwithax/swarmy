package update

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestClassifyDistro(t *testing.T) {
	tests := []struct {
		name     string
		id       string
		idLike   string
		expected DistroType
	}{
		{"Alpine", "alpine", "", DistroAlpine},
		{"Arch", "arch", "", DistroArch},
		{"Manjaro", "manjaro", "arch", DistroArch},
		{"Debian", "debian", "", DistroDebian},
		{"Ubuntu", "ubuntu", "debian", DistroDebian},
		{"Linux Mint", "linuxmint", "ubuntu debian", DistroDebian},
		{"Pop!_OS", "pop", "ubuntu debian", DistroDebian},
		{"Fedora", "fedora", "", DistroRedHat},
		{"RHEL", "rhel", "fedora", DistroRedHat},
		{"CentOS", "centos", "rhel fedora", DistroRedHat},
		{"Rocky Linux", "rocky", "rhel centos fedora", DistroRedHat},
		{"openSUSE", "opensuse-leap", "suse", DistroSUSE},
		{"Unknown", "unknown", "", DistroUnknown},
		{"Debian via ID_LIKE", "custom", "debian", DistroDebian},
		{"Fedora via ID_LIKE", "custom", "fedora", DistroRedHat},
		{"Arch via ID_LIKE", "custom", "arch", DistroArch},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := classifyDistro(tt.id, tt.idLike)
			require.Equal(t, tt.expected, result)
		})
	}
}

func TestDistroInfoPreferredPackageFormat(t *testing.T) {
	tests := []struct {
		distro   DistroInfo
		expected string
	}{
		{DistroInfo{Type: DistroDebian}, "deb"},
		{DistroInfo{Type: DistroRedHat}, "rpm"},
		{DistroInfo{Type: DistroSUSE}, "rpm"},
		{DistroInfo{Type: DistroAlpine}, "apk"},
		{DistroInfo{Type: DistroArch}, "arch_pkg"},
		{DistroInfo{Type: DistroUnknown}, "archive"},
	}

	for _, tt := range tests {
		t.Run(string(tt.distro.Type), func(t *testing.T) {
			result := tt.distro.PreferredPackageFormat()
			require.Equal(t, tt.expected, result)
		})
	}
}

func TestParseOSRelease(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a mock os-release file.
	content := `NAME="Ubuntu"
VERSION_ID="22.04"
ID=ubuntu
ID_LIKE=debian
PRETTY_NAME="Ubuntu 22.04.3 LTS"
`
	osReleasePath := filepath.Join(tmpDir, "os-release")
	err := os.WriteFile(osReleasePath, []byte(content), 0o644)
	require.NoError(t, err)

	info, ok := parseOSRelease(osReleasePath)
	require.True(t, ok)
	require.Equal(t, "ubuntu", info.ID)
	require.Equal(t, "debian", info.IDLike)
	require.Equal(t, "Ubuntu", info.Name)
	require.Equal(t, "22.04", info.Version)
	require.Equal(t, DistroDebian, info.Type)
}

func TestParseOSReleaseNotFound(t *testing.T) {
	info, ok := parseOSRelease("/nonexistent/path")
	require.False(t, ok)
	// When parseOSRelease fails, it returns an empty Type field.
	require.Equal(t, DistroType(""), info.Type)
}

func TestDetectDistroNonLinux(t *testing.T) {
	if runtime.GOOS == "linux" {
		t.Skip("Skipping on Linux")
	}

	info := DetectDistro()
	require.Equal(t, DistroUnknown, info.Type)
}

func TestDistroInfoString(t *testing.T) {
	info := DistroInfo{
		Type:    DistroDebian,
		Name:    "Ubuntu",
		Version: "22.04",
	}
	require.Equal(t, "debian (Ubuntu 22.04)", info.String())

	unknown := DistroInfo{Type: DistroUnknown}
	require.Equal(t, "unknown", unknown.String())
}
