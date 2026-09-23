package provisioner

import (
	"fmt"
	"strings"
)

// shellQuote wraps s in single quotes for a POSIX shell, escaping any
// embedded single quotes.
func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

// psQuote wraps s in single quotes for PowerShell, escaping any embedded
// single quotes by doubling them.
func psQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "''") + "'"
}

// powershellCmd wraps a script for execution over an SSH exec channel on a
// Windows target, regardless of whether the account's default shell is
// cmd.exe or PowerShell.
func powershellCmd(script string) string {
	return "powershell -NoProfile -NonInteractive -Command " + psQuote(script)
}

// linuxDepsInstallCmd builds a shell snippet that installs OS packages via
// whichever package manager is present, entirely non-interactively — no
// SSH session has a controlling TTY, so without this apt-get's debconf
// prompts (and needrestart's service-restart prompts) either hang or fail
// with a non-zero exit. debianAlpinePkgs is used for apt-get/apk (which
// share package names, e.g. "iproute2"); rpmPkgs is used for dnf/yum
// (which name some packages differently, e.g. "iproute").
func linuxDepsInstallCmd(debianAlpinePkgs, rpmPkgs string) string {
	return fmt.Sprintf(`
export DEBIAN_FRONTEND=noninteractive
export NEEDRESTART_MODE=a
if command -v apt-get >/dev/null 2>&1; then
  apt-get update -y && apt-get install -y -o Dpkg::Options::="--force-confdef" -o Dpkg::Options::="--force-confold" %s
elif command -v dnf >/dev/null 2>&1; then
  dnf install -y %s
elif command -v yum >/dev/null 2>&1; then
  yum install -y %s
elif command -v apk >/dev/null 2>&1; then
  apk add --no-cache %s
fi`, debianAlpinePkgs, rpmPkgs, rpmPkgs, debianAlpinePkgs)
}
