package provisioner

import "fmt"

// netclientReleaseURL builds the GitHub Releases download URL for a given
// netclient version/OS/arch, matching the pattern used by netclient's own
// `use` command (netclient/functions/use_version.go). An empty version
// resolves to GitHub's "latest release" URL form
// (releases/latest/download/<asset>, vs. releases/download/<tag>/<asset>
// for a pinned version) — used when auto_update is enabled instead of a
// pinned netclient_version.
func netclientReleaseURL(version string, info Info) string {
	assetName := fmt.Sprintf("netclient-%s-%s", info.OS, info.Arch)
	if info.OS == OSWindows {
		assetName = fmt.Sprintf("netclient-windows-%s.exe", info.Arch)
	}
	if version == "" {
		return fmt.Sprintf("https://github.com/gravitl/netclient/releases/latest/download/%s", assetName)
	}
	return fmt.Sprintf("https://github.com/gravitl/netclient/releases/download/%s/%s", version, assetName)
}

// InstallAndJoin downloads the given netclient version (or the latest
// release, if version is empty — see netclientReleaseURL), installs it as
// an OS service, and joins the network(s) covered by token, naming the
// host `name`. The binary is staged at a temp path and invoked directly
// for both `install` and `join` — netclient's own `install` step is what
// copies itself to its real OS-specific install location, so the staged
// binary's path doesn't need to match that location.
func InstallAndJoin(c *Client, info Info, version, token, name string) error {
	switch info.OS {
	case OSWindows:
		return installAndJoinWindows(c, info, version, token, name)
	default:
		return installAndJoinUnix(c, info, version, token, name)
	}
}

func installAndJoinUnix(c *Client, info Info, version, token, name string) error {
	const staged = "/tmp/netclient-install"

	depsCmd := ""
	switch info.OS {
	case OSDarwin:
		depsCmd = "command -v brew >/dev/null 2>&1 && brew install wireguard-tools || true"
	default: // Linux
		depsCmd = linuxDepsInstallCmd("wireguard-tools iproute2 iptables nftables", "wireguard-tools iproute iptables nftables")
	}

	script := fmt.Sprintf(`set -e
%s
curl -fsSL -o %s "%s"
chmod +x %s
%s install
%s join -t %s -o %s
`, depsCmd, staged, netclientReleaseURL(version, info), staged, staged, staged, shellQuote(token), shellQuote(name))

	_, err := c.Run(script)
	return err
}

func installAndJoinWindows(c *Client, info Info, version, token, name string) error {
	// Windows netclient builds bundle their own WireGuard driver (wintun) —
	// no separate WireGuard-for-Windows app install is needed for netclient
	// itself (unlike the ext-client path, which does need it — see
	// wireguard.go).
	script := fmt.Sprintf(`
$ErrorActionPreference = "Stop"
Invoke-WebRequest -Uri "%s" -OutFile "$env:TEMP\netclient.exe"
& "$env:TEMP\netclient.exe" install
& "$env:TEMP\netclient.exe" join -t %s -o %s
`, netclientReleaseURL(version, info), psQuote(token), psQuote(name))

	_, err := c.Run(powershellCmd(script))
	return err
}

// UseVersion swaps an already-installed netclient to a specific release
// version in place (netclient's own `use` command — netclient/functions/
// use_version.go: stops the daemon, replaces the running binary, restarts
// it). Used to change netclient_version without needing to reinstall or
// rejoin the network. Tries the bare `netclient` command (on PATH after
// install) first, falling back to the staged temp copy from the original
// install, same pattern as Uninstall.
func UseVersion(c *Client, info Info, version string) error {
	switch info.OS {
	case OSWindows:
		_, err := c.Run(powershellCmd(fmt.Sprintf(`& netclient use %s`, psQuote(version))))
		if err != nil {
			_, err = c.Run(powershellCmd(fmt.Sprintf(`& "$env:TEMP\netclient.exe" use %s`, psQuote(version))))
		}
		return err
	default:
		if _, err := c.Run(fmt.Sprintf("netclient use %s", shellQuote(version))); err != nil {
			_, err = c.Run(fmt.Sprintf("/tmp/netclient-install use %s", shellQuote(version)))
			return err
		}
		return nil
	}
}

// Uninstall removes netclient from the target machine, which cascades
// deletion of the corresponding Host (and all its Nodes) server-side —
// netclient's own `uninstall` iterates every joined server and sends a
// DeleteHost update, so no separate per-network `leave` is needed first.
func Uninstall(c *Client, info Info) error {
	switch info.OS {
	case OSWindows:
		// Try PATH first, fall back to the staged temp copy in case PATH
		// wasn't updated by install.
		_, err := c.Run(powershellCmd(`& netclient uninstall`))
		if err != nil {
			_, err = c.Run(powershellCmd(`& "$env:TEMP\netclient.exe" uninstall`))
		}
		return err
	default:
		if _, err := c.Run("netclient uninstall"); err != nil {
			_, err = c.Run("/tmp/netclient-install uninstall")
			return err
		}
		return nil
	}
}
