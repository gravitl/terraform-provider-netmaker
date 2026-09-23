// Package provisioner implements SSH-based deployment of netclient
// (netmaker_device) and WireGuard/ext-client configs (netmaker_ext_client)
// onto real machines, across Linux, macOS, and Windows targets.
package provisioner

import (
	"bytes"
	"fmt"
	"time"

	"golang.org/x/crypto/ssh"
)

// Config describes how to reach a target machine over SSH.
type Config struct {
	Host       string
	Port       int64
	User       string
	PrivateKey string // PEM-encoded private key, optional if Password is set
	Password   string // optional if PrivateKey is set
}

// Client wraps an established SSH connection to a target machine.
type Client struct {
	conn *ssh.Client
}

// Connect dials the target machine described by cfg.
//
// Known limitation: host key verification is not performed
// (ssh.InsecureIgnoreHostKey) — there is no way today to supply a known
// host key via the resource schema. This matches the default behavior of
// Terraform's own built-in `connection` block provisioners and most
// infra-automation tools when no host key is pinned, but is worth
// revisiting (e.g. an optional `host_key` attribute) before this is used
// against untrusted networks.
func Connect(cfg Config) (*Client, error) {
	if cfg.PrivateKey == "" && cfg.Password == "" {
		return nil, fmt.Errorf("ssh: at least one of private_key or password must be set")
	}

	var auths []ssh.AuthMethod
	if cfg.PrivateKey != "" {
		signer, err := ssh.ParsePrivateKey([]byte(cfg.PrivateKey))
		if err != nil {
			return nil, fmt.Errorf("ssh: parsing private key: %w", err)
		}
		auths = append(auths, ssh.PublicKeys(signer))
	}
	if cfg.Password != "" {
		auths = append(auths, ssh.Password(cfg.Password))
	}

	port := cfg.Port
	if port == 0 {
		port = 22
	}

	conn, err := ssh.Dial("tcp", fmt.Sprintf("%s:%d", cfg.Host, port), &ssh.ClientConfig{
		User:            cfg.User,
		Auth:            auths,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(), //nolint:gosec // see doc comment above
		Timeout:         30 * time.Second,
	})
	if err != nil {
		return nil, fmt.Errorf("ssh: connecting to %s@%s:%d: %w", cfg.User, cfg.Host, port, err)
	}
	return &Client{conn: conn}, nil
}

func (c *Client) Close() error {
	return c.conn.Close()
}

// Run executes a single command on the target machine and returns its
// combined stdout/stderr. Returns an error (including that combined
// output) if the command exits non-zero.
func (c *Client) Run(cmd string) (string, error) {
	session, err := c.conn.NewSession()
	if err != nil {
		return "", fmt.Errorf("ssh: opening session: %w", err)
	}
	defer session.Close()

	var out bytes.Buffer
	session.Stdout = &out
	session.Stderr = &out

	if err := session.Run(cmd); err != nil {
		return out.String(), fmt.Errorf("ssh: command %q failed: %w: %s", cmd, err, out.String())
	}
	return out.String(), nil
}

// WriteFile writes content to path on the target machine by piping it
// through stdin to writeCmd (e.g. "cat > /etc/wireguard/wg0.conf" on
// Unix, or a PowerShell Set-Content invocation on Windows) — avoids
// depending on SFTP for what is otherwise a single small text file.
func (c *Client) WriteFile(writeCmd, content string) error {
	session, err := c.conn.NewSession()
	if err != nil {
		return fmt.Errorf("ssh: opening session: %w", err)
	}
	defer session.Close()

	session.Stdin = bytes.NewBufferString(content)
	var out bytes.Buffer
	session.Stdout = &out
	session.Stderr = &out

	if err := session.Run(writeCmd); err != nil {
		return fmt.Errorf("ssh: writing file via %q failed: %w: %s", writeCmd, err, out.String())
	}
	return nil
}
