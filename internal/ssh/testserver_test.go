package ssh

import (
	"crypto/ed25519"
	"crypto/rand"
	"io"
	"net"
	"testing"

	"golang.org/x/crypto/ssh"
)

// fakeSSHServer is a minimal in-process SSH server used to exercise Pool
// against a real network connection without depending on Docker/an actual
// sshd. It supports password auth and "exec" requests only.
type fakeSSHServer struct {
	Addr     string
	HostKey  ssh.PublicKey
	User     string
	Password string
}

// commandOutcome maps a command string to the output/exit status the fake
// server should return for it.
type commandOutcome struct {
	stdout   string
	stderr   string
	exitCode uint32
}

func startFakeSSHServer(t *testing.T, outcomes map[string]commandOutcome) *fakeSSHServer {
	t.Helper()

	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate host key: %v", err)
	}
	signer, err := ssh.NewSignerFromSigner(priv)
	if err != nil {
		t.Fatalf("signer from key: %v", err)
	}

	const user = "testuser"
	const password = "testpass"

	config := &ssh.ServerConfig{
		PasswordCallback: func(c ssh.ConnMetadata, pass []byte) (*ssh.Permissions, error) {
			if c.User() == user && string(pass) == password {
				return nil, nil
			}
			return nil, errAuthFailed
		},
	}
	config.AddHostKey(signer)

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	t.Cleanup(func() { _ = listener.Close() })

	go func() {
		for {
			nConn, err := listener.Accept()
			if err != nil {
				return
			}
			go handleFakeConn(nConn, config, outcomes)
		}
	}()

	return &fakeSSHServer{
		Addr:     listener.Addr().String(),
		HostKey:  signer.PublicKey(),
		User:     user,
		Password: password,
	}
}

var errAuthFailed = &authError{"authentication failed"}

type authError struct{ msg string }

func (e *authError) Error() string { return e.msg }

func handleFakeConn(nConn net.Conn, config *ssh.ServerConfig, outcomes map[string]commandOutcome) {
	conn, chans, reqs, err := ssh.NewServerConn(nConn, config)
	if err != nil {
		return
	}
	defer conn.Close()
	go ssh.DiscardRequests(reqs)

	for newChannel := range chans {
		if newChannel.ChannelType() != "session" {
			_ = newChannel.Reject(ssh.UnknownChannelType, "unsupported channel type")
			continue
		}
		channel, requests, err := newChannel.Accept()
		if err != nil {
			continue
		}
		go serveChannel(channel, requests, outcomes)
	}
}

func serveChannel(channel ssh.Channel, requests <-chan *ssh.Request, outcomes map[string]commandOutcome) {
	defer channel.Close()
	for req := range requests {
		if req.Type != "exec" {
			if req.WantReply {
				_ = req.Reply(false, nil)
			}
			continue
		}

		var payload struct{ Value string }
		_ = ssh.Unmarshal(req.Payload, &payload)
		if req.WantReply {
			_ = req.Reply(true, nil)
		}

		outcome, ok := outcomes[payload.Value]
		if !ok {
			outcome = commandOutcome{stderr: "command not found", exitCode: 127}
		}
		if outcome.stdout != "" {
			_, _ = io.WriteString(channel, outcome.stdout)
		}
		if outcome.stderr != "" {
			_, _ = io.WriteString(channel.Stderr(), outcome.stderr)
		}
		_, _ = channel.SendRequest("exit-status", false, ssh.Marshal(&struct{ Status uint32 }{outcome.exitCode}))
		return
	}
}
