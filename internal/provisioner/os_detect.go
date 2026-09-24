package provisioner

import (
	"fmt"
	"strings"
)

// OSFamily is a detected target OS, matching Go's GOOS naming.
type OSFamily string

const (
	OSLinux   OSFamily = "linux"
	OSDarwin  OSFamily = "darwin"
	OSWindows OSFamily = "windows"
)

// Info describes a detected target machine.
type Info struct {
	OS   OSFamily
	Arch string // netclient release naming: amd64, arm64, armv6, armv7, ...
}

// Detect probes the target machine to determine its OS family and CPU
// architecture. `uname` exists on Linux/macOS; its absence (non-Windows
// shells always have it) is taken as a Windows target, since Windows'
// default SSH shells (cmd.exe/PowerShell) don't recognize it.
func Detect(c *Client) (Info, error) {
	unameS, err := c.Run("uname -s")
	if err != nil {
		return Info{OS: OSWindows, Arch: "amd64"}, nil
	}

	var osFamily OSFamily
	switch {
	case strings.Contains(unameS, "Linux"):
		osFamily = OSLinux
	case strings.Contains(unameS, "Darwin"):
		osFamily = OSDarwin
	default:
		return Info{}, errUnsupportedOS(unameS)
	}

	unameM, err := c.Run("uname -m")
	if err != nil {
		return Info{}, err
	}
	return Info{OS: osFamily, Arch: normalizeArch(strings.TrimSpace(unameM))}, nil
}

func normalizeArch(unameM string) string {
	switch unameM {
	case "x86_64", "amd64":
		return "amd64"
	case "aarch64", "arm64":
		return "arm64"
	case "armv7l":
		return "armv7"
	case "armv6l":
		return "armv6"
	default:
		return unameM
	}
}

func errUnsupportedOS(unameS string) error {
	return &unsupportedOSError{unameS: strings.TrimSpace(unameS)}
}

type unsupportedOSError struct{ unameS string }

func (e *unsupportedOSError) Error() string {
	return fmt.Sprintf("provisioner: unrecognized target OS from `uname -s`: %q", e.unameS)
}
