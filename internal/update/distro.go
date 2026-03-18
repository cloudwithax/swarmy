package update

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// DistroType represents the type of Linux distribution.
type DistroType string

const (
	// DistroUnknown represents an unknown or non-Linux distro.
	DistroUnknown DistroType = "unknown"
	// DistroDebian represents Debian-based distros (Debian, Ubuntu, etc.).
	DistroDebian DistroType = "debian"
	// DistroRedHat represents RedHat-based distros (RHEL, Fedora, CentOS, etc.).
	DistroRedHat DistroType = "redhat"
	// DistroAlpine represents Alpine Linux.
	DistroAlpine DistroType = "alpine"
	// DistroArch represents Arch Linux and derivatives.
	DistroArch DistroType = "arch"
	// DistroSUSE represents openSUSE.
	DistroSUSE DistroType = "suse"
)

// DistroInfo contains information about the detected Linux distribution.
type DistroInfo struct {
	Type    DistroType
	ID      string
	IDLike  string
	Name    string
	Version string
}

// String returns a string representation of the distro info.
func (d DistroInfo) String() string {
	if d.Type == DistroUnknown {
		return "unknown"
	}
	return fmt.Sprintf("%s (%s %s)", d.Type, d.Name, d.Version)
}

// PreferredPackageFormat returns the preferred package format for this distro.
func (d DistroInfo) PreferredPackageFormat() string {
	switch d.Type {
	case DistroDebian:
		return "deb"
	case DistroRedHat, DistroSUSE:
		return "rpm"
	case DistroAlpine:
		return "apk"
	case DistroArch:
		return "arch_pkg"
	default:
		return "archive"
	}
}

// DetectDistro detects the Linux distribution.
// Returns DistroUnknown on non-Linux platforms.
func DetectDistro() DistroInfo {
	if runtime.GOOS != "linux" {
		return DistroInfo{Type: DistroUnknown}
	}

	// Try /etc/os-release first (systemd standard).
	if info, ok := parseOSRelease("/etc/os-release"); ok {
		return info
	}

	// Try /usr/lib/os-release as fallback.
	if info, ok := parseOSRelease("/usr/lib/os-release"); ok {
		return info
	}

	// Try legacy detection methods.
	return detectLegacy()
}

// parseOSRelease parses an os-release file.
func parseOSRelease(path string) (DistroInfo, bool) {
	file, err := os.Open(path)
	if err != nil {
		return DistroInfo{}, false
	}
	defer file.Close()

	info := DistroInfo{Type: DistroUnknown}
	scanner := bufio.NewScanner(file)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}

		key := parts[0]
		value := strings.Trim(parts[1], `"`)

		switch key {
		case "ID":
			info.ID = value
		case "ID_LIKE":
			info.IDLike = value
		case "NAME":
			info.Name = value
		case "VERSION_ID":
			info.Version = value
		}
	}

	info.Type = classifyDistro(info.ID, info.IDLike)
	return info, true
}

// classifyDistro classifies a distro based on its ID and ID_LIKE fields.
func classifyDistro(id, idLike string) DistroType {
	id = strings.ToLower(id)
	idLike = strings.ToLower(idLike)

	// Check ID first.
	switch id {
	case "alpine":
		return DistroAlpine
	case "arch", "manjaro", "endeavouros", "garuda":
		return DistroArch
	case "debian", "ubuntu", "linuxmint", "pop", "elementary", "zorin":
		return DistroDebian
	case "fedora", "rhel", "centos", "rocky", "almalinux":
		return DistroRedHat
	case "opensuse", "opensuse-leap", "opensuse-tumbleweed", "sles":
		return DistroSUSE
	}

	// Check ID_LIKE as fallback.
	if strings.Contains(idLike, "debian") || strings.Contains(idLike, "ubuntu") {
		return DistroDebian
	}
	if strings.Contains(idLike, "fedora") || strings.Contains(idLike, "rhel") || strings.Contains(idLike, "centos") {
		return DistroRedHat
	}
	if strings.Contains(idLike, "arch") {
		return DistroArch
	}
	if strings.Contains(idLike, "suse") {
		return DistroSUSE
	}

	return DistroUnknown
}

// detectLegacy tries to detect the distro using legacy methods.
func detectLegacy() DistroInfo {
	// Check for Alpine (has /etc/alpine-release).
	if _, err := os.Stat("/etc/alpine-release"); err == nil {
		return DistroInfo{
			Type: DistroAlpine,
			ID:   "alpine",
			Name: "Alpine Linux",
		}
	}

	// Check for Arch (has /etc/arch-release).
	if _, err := os.Stat("/etc/arch-release"); err == nil {
		return DistroInfo{
			Type: DistroArch,
			ID:   "arch",
			Name: "Arch Linux",
		}
	}

	// Check for Debian (has /etc/debian_version).
	if _, err := os.Stat("/etc/debian_version"); err == nil {
		return DistroInfo{
			Type: DistroDebian,
			ID:   "debian",
			Name: "Debian",
		}
	}

	// Check for Fedora (has /etc/fedora-release).
	if _, err := os.Stat("/etc/fedora-release"); err == nil {
		return DistroInfo{
			Type: DistroRedHat,
			ID:   "fedora",
			Name: "Fedora",
		}
	}

	// Check for RedHat (has /etc/redhat-release).
	if _, err := os.Stat("/etc/redhat-release"); err == nil {
		return DistroInfo{
			Type: DistroRedHat,
			ID:   "rhel",
			Name: "Red Hat Enterprise Linux",
		}
	}

	// Check for openSUSE (has /etc/SuSE-release or /etc/os-release with suse).
	if _, err := os.Stat("/etc/SuSE-release"); err == nil {
		return DistroInfo{
			Type: DistroSUSE,
			ID:   "opensuse",
			Name: "openSUSE",
		}
	}

	return DistroInfo{Type: DistroUnknown}
}

// HasPackageManager checks if the system has a specific package manager installed.
func HasPackageManager(manager string) bool {
	_, err := os.Stat(filepath.Join("/usr/bin", manager))
	if err == nil {
		return true
	}
	_, err = os.Stat(filepath.Join("/bin", manager))
	return err == nil
}

// DetectPackageManager detects which package manager is available.
func DetectPackageManager() string {
	managers := []string{"apt", "apt-get", "dnf", "yum", "pacman", "apk", "zypper"}
	for _, m := range managers {
		if HasPackageManager(m) {
			return m
		}
	}
	return ""
}
