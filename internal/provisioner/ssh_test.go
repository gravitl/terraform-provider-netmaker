package provisioner

import (
	"crypto/ed25519"
	"crypto/rand"
	"net"
	"strings"
	"testing"

	"golang.org/x/crypto/ssh"
)

// startTestSSHServer runs an in-process SSH server on localhost that
// answers every exec request with the given stdout/stderr and exit status,
// and returns a connected Client.
func startTestSSHServer(t *testing.T, stdout, stderr string, exitStatus uint32) *Client {
	t.Helper()

	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	signer, err := ssh.NewSignerFromKey(priv)
	if err != nil {
		t.Fatal(err)
	}
	config := &ssh.ServerConfig{NoClientAuth: true}
	config.AddHostKey(signer)

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { ln.Close() })

	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go serveTestConn(conn, config, stdout, stderr, exitStatus)
		}
	}()

	client, err := Connect(Config{
		Host:     "127.0.0.1",
		Port:     int64(ln.Addr().(*net.TCPAddr).Port),
		User:     "test",
		Password: "unused",
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { client.Close() })
	return client
}

func serveTestConn(conn net.Conn, config *ssh.ServerConfig, stdout, stderr string, exitStatus uint32) {
	_, chans, reqs, err := ssh.NewServerConn(conn, config)
	if err != nil {
		return
	}
	go ssh.DiscardRequests(reqs)

	for newCh := range chans {
		if newCh.ChannelType() != "session" {
			newCh.Reject(ssh.UnknownChannelType, "session only")
			continue
		}
		ch, chReqs, err := newCh.Accept()
		if err != nil {
			return
		}
		go func() {
			for req := range chReqs {
				if req.Type != "exec" {
					req.Reply(false, nil)
					continue
				}
				req.Reply(true, nil)
				if stdout != "" {
					ch.Write([]byte(stdout))
				}
				if stderr != "" {
					ch.Stderr().Write([]byte(stderr))
				}
				ch.SendRequest("exit-status", false, ssh.Marshal(struct{ Status uint32 }{exitStatus}))
				ch.Close()
				return
			}
		}()
	}
}

// Regression test: Run must return a command's output even when it writes
// only to stdout. Sharing one bytes.Buffer between the stdout and stderr
// copy goroutines lost that output (the stderr copy finishing after the
// stdout copy reset the buffer's length), which made OS detection see an
// empty `uname -s`.
func TestRunKeepsStdoutOutput(t *testing.T) {
	client := startTestSSHServer(t, "Linux\n", "", 0)

	for i := 0; i < 300; i++ {
		out, err := client.Run("uname -s")
		if err != nil {
			t.Fatalf("run %d: %v", i, err)
		}
		if out != "Linux\n" {
			t.Fatalf("run %d: output = %q, want %q (output lost)", i, out, "Linux\n")
		}
	}
}

func TestRunCombinesStdoutAndStderr(t *testing.T) {
	client := startTestSSHServer(t, "out\n", "err\n", 0)

	out, err := client.Run("cmd")
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != len("out\nerr\n") || out != "out\nerr\n" && out != "err\nout\n" {
		t.Errorf("output = %q, want both stdout and stderr", out)
	}
}

func TestRunFailureIncludesOutput(t *testing.T) {
	client := startTestSSHServer(t, "", "boom\n", 3)

	out, err := client.Run("cmd")
	if err == nil {
		t.Fatal("want an error for a non-zero exit")
	}
	if out != "boom\n" {
		t.Errorf("output = %q, want %q", out, "boom\n")
	}
	if got := err.Error(); !strings.Contains(got, "boom") {
		t.Errorf("error %q should include the command's output", got)
	}
}
