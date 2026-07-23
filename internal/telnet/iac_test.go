package telnet

import (
	"net"
	"testing"
)

// pipeConn gives iacFilter a real net.Conn to write replies to, backed by
// an in-memory pipe, so tests can inspect exactly what the filter sends
// back for each negotiation request.
func pipeConn(t *testing.T) (client, remote net.Conn) {
	t.Helper()
	c, r := net.Pipe()
	t.Cleanup(func() { _ = c.Close(); _ = r.Close() })
	return c, r
}

func TestIACFilter_PlainTextPassesThrough(t *testing.T) {
	client, _ := pipeConn(t)
	f := &iacFilter{conn: client}

	got := f.feed([]byte("hello world"))
	if string(got) != "hello world" {
		t.Errorf("got %q, want %q", got, "hello world")
	}
}

func TestIACFilter_EscapedIACByte(t *testing.T) {
	client, _ := pipeConn(t)
	f := &iacFilter{conn: client}

	got := f.feed([]byte{'a', iacByte, iacByte, 'b'})
	want := []byte{'a', iacByte, 'b'}
	if string(got) != string(want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestIACFilter_StripsNegotiationAndReplies(t *testing.T) {
	client, remote := pipeConn(t)
	f := &iacFilter{conn: client}

	replies := make(chan []byte, 1)
	go func() {
		buf := make([]byte, 16)
		n, _ := remote.Read(buf)
		replies <- buf[:n]
	}()

	// IAC DO ECHO ("please echo"), surrounded by plain text.
	input := []byte{'x', iacByte, doByte, 1, 'y'}
	got := f.feed(input)
	if string(got) != "xy" {
		t.Errorf("plain output = %q, want %q", got, "xy")
	}

	reply := <-replies
	want := []byte{iacByte, wontByte, 1}
	if string(reply) != string(want) {
		t.Errorf("reply = %v, want %v (IAC WONT ECHO)", reply, want)
	}
}

func TestIACFilter_NegotiationSplitAcrossFeeds(t *testing.T) {
	client, remote := pipeConn(t)
	f := &iacFilter{conn: client}

	replies := make(chan []byte, 1)
	go func() {
		buf := make([]byte, 16)
		n, _ := remote.Read(buf)
		replies <- buf[:n]
	}()

	// Split IAC WILL SUPPRESS-GO-AHEAD across three feed() calls.
	got1 := f.feed([]byte{'a', iacByte})
	got2 := f.feed([]byte{willByte})
	got3 := f.feed([]byte{3, 'b'})

	if string(got1)+string(got2)+string(got3) != "ab" {
		t.Errorf("plain output = %q%q%q, want %q", got1, got2, got3, "ab")
	}

	reply := <-replies
	want := []byte{iacByte, dontByte, 3}
	if string(reply) != string(want) {
		t.Errorf("reply = %v, want %v (IAC DONT SUPPRESS-GO-AHEAD)", reply, want)
	}
}

func TestIACFilter_SubnegotiationDiscarded(t *testing.T) {
	client, _ := pipeConn(t)
	f := &iacFilter{conn: client}

	// IAC SB <garbage> IAC SE, surrounded by plain text.
	input := []byte{'a', iacByte, sbByte, 1, 2, 3, iacByte, seByte, 'b'}
	got := f.feed(input)
	if string(got) != "ab" {
		t.Errorf("got %q, want %q", got, "ab")
	}
}
