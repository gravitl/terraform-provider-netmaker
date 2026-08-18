package provisioner

import "fmt"

// InterfaceName picks a stable WireGuard interface/tunnel name from an ext
// client ID (both wg-quick and WireGuard-for-Windows require a short,
// filesystem-safe name).
func InterfaceName(clientID string) string {
	name := clientID
	if len(name) > 12 {
		name = name[:12]
	}
	return "nm-" + name
}

// ApplyExtClientConfig installs WireGuard (if needed) on the target
// machine and brings up an interface/tunnel from confContent (the
// server-rendered wg-quick config text from
// Client.GetExtClientConfigFile). Unlike netclient, there is no
// netmaker-specific tooling for ext clients at all — this is plain
// wg-quick on Linux/macOS, and WireGuard's own tunnel service on Windows
// (which doesn't use wg-quick).
func ApplyExtClientConfig(c *Client, info Info, ifaceName, confContent string) error {
	switch info.OS {
	case OSWindows:
		return applyExtClientWindows(c, ifaceName, confContent)
	case OSDarwin:
		return applyExtClientUnix(c, ifaceName, confContent, "command -v brew >/dev/null 2>&1 && brew install wireguard-tools || true")
	default: // Linux
		return applyExtClientUnix(c, ifaceName, confContent, linuxDepsInstallCmd("wireguard-tools", "wireguard-tools"))
	}
}

func applyExtClientUnix(c *Client, ifaceName, confContent, depsCmd string) error {
	if _, err := c.Run(fmt.Sprintf("set -e\n%s\nmkdir -p /etc/wireguard", depsCmd)); err != nil {
		return err
	}
	confPath := fmt.Sprintf("/etc/wireguard/%s.conf", ifaceName)
	if err := c.WriteFile(fmt.Sprintf("cat > %s && chmod 600 %s", confPath, confPath), confContent); err != nil {
		return err
	}
	_, err := c.Run(fmt.Sprintf("wg-quick up %s", ifaceName))
	return err
}

func applyExtClientWindows(c *Client, ifaceName, confContent string) error {
	// Assumes WireGuard for Windows (https://www.wireguard.com/install/) is
	// already present on the target — its MSI installer isn't
	// silently-scriptable in a way we can reliably automate here, so
	// unlike netclient's bundled driver, this is a documented
	// precondition rather than something this provisioner installs.
	// `wireguard.exe /installtunnelservice <path>` reads a plaintext
	// wg-quick-style .conf from any path and takes ownership of it (moving
	// it into its own DPAPI-encrypted store) — stage it under $env:TEMP.
	stagedPath := fmt.Sprintf(`$env:TEMP\%s.conf`, ifaceName)
	if err := c.WriteFile(powershellCmd(fmt.Sprintf(`$input | Set-Content -Path "%s" -Encoding ascii`, stagedPath)), confContent); err != nil {
		return err
	}
	_, err := c.Run(powershellCmd(fmt.Sprintf(`& wireguard.exe /installtunnelservice "%s"`, stagedPath)))
	return err
}

// TeardownExtClientConfig removes the interface/tunnel created by
// ApplyExtClientConfig.
func TeardownExtClientConfig(c *Client, info Info, ifaceName string) error {
	switch info.OS {
	case OSWindows:
		_, err := c.Run(powershellCmd(fmt.Sprintf(`& wireguard.exe /uninstalltunnelservice "%s"`, ifaceName)))
		return err
	default:
		_, err := c.Run(fmt.Sprintf("wg-quick down %s || true; rm -f /etc/wireguard/%s.conf", ifaceName, ifaceName))
		return err
	}
}
