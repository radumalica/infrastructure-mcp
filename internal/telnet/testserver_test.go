package telnet

import (
	"bufio"
	"net"
	"strings"
	"testing"
)

// fakeTelnetServer is a minimal in-process Telnet server used to exercise
// Pool against a real network connection: login prompt, password prompt,
// then a line-oriented command loop with scripted responses. It does not
// send any IAC negotiation itself (many embedded devices don't either),
// but the client's IAC filter is exercised separately in iac_test.go.
type fakeTelnetServer struct {
	Addr     string
	User     string
	Password string
}

func startFakeTelnetServer(t *testing.T, responses map[string]string) *fakeTelnetServer {
	t.Helper()

	const user = "admin"
	const password = "letmein"

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	t.Cleanup(func() { _ = listener.Close() })

	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			go serveFakeTelnet(conn, user, password, responses)
		}
	}()

	return &fakeTelnetServer{Addr: listener.Addr().String(), User: user, Password: password}
}

func serveFakeTelnet(conn net.Conn, user, password string, responses map[string]string) {
	defer conn.Close()
	r := bufio.NewReader(conn)

	_, _ = conn.Write([]byte("login: "))
	gotUser, err := r.ReadString('\n')
	if err != nil {
		return
	}
	gotUser = strings.TrimSpace(gotUser)

	_, _ = conn.Write([]byte("password: "))
	gotPass, err := r.ReadString('\n')
	if err != nil {
		return
	}
	gotPass = strings.TrimSpace(gotPass)

	if gotUser != user || gotPass != password {
		// Real devices typically re-show the login prompt rather than
		// dropping the connection, so the client's post-login idle read
		// sees "...login:" and can detect the failure via ErrLoginFailed
		// instead of a raw EOF. Stay connected and idle to mirror that.
		_, _ = conn.Write([]byte("\r\nAccess denied\r\nlogin: "))
		_, _ = r.ReadString('\n')
		return
	}

	_, _ = conn.Write([]byte("\r\nWelcome\r\ndevice> "))

	for {
		line, err := r.ReadString('\n')
		if err != nil {
			return
		}
		cmd := strings.TrimSpace(line)
		resp, ok := responses[cmd]
		if !ok {
			resp = "% Unknown command"
		}
		_, _ = conn.Write([]byte(cmd + "\r\n" + resp + "\r\ndevice> "))
	}
}
