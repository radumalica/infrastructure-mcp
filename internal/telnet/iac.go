package telnet

import "net"

// Telnet protocol bytes (RFC 854).
const (
	iacByte  byte = 255
	doByte   byte = 253
	dontByte byte = 254
	willByte byte = 251
	wontByte byte = 252
	sbByte   byte = 250
	seByte   byte = 240
)

type iacState int

const (
	stateNormal iacState = iota
	stateGotIAC
	stateGotCommand // saw WILL/WONT/DO/DONT, waiting for the option byte
	stateInSubneg   // inside an IAC SB ... IAC SE block
	stateSubnegIAC  // saw IAC while inside a subnegotiation block
)

// iacFilter strips Telnet option-negotiation sequences from a byte
// stream, replying to every negotiation request by refusing it (WONT for
// WILL, DONT for DO) so the remote side falls back to plain character
// mode — the mode virtually every embedded device CLI expects from a
// dumb client. State persists across reads, since a negotiation sequence
// can be split across two Read() calls.
type iacFilter struct {
	state      iacState
	pendingCmd byte
	conn       net.Conn
}

// feed processes raw bytes just read from the connection, writing any
// required negotiation replies to conn as a side effect, and returns the
// plain (non-protocol) bytes for the caller to accumulate.
func (f *iacFilter) feed(data []byte) []byte {
	plain := make([]byte, 0, len(data))

	for _, b := range data {
		switch f.state {
		case stateNormal:
			if b == iacByte {
				f.state = stateGotIAC
				continue
			}
			plain = append(plain, b)

		case stateGotIAC:
			switch b {
			case iacByte:
				// Escaped literal 0xFF.
				plain = append(plain, iacByte)
				f.state = stateNormal
			case willByte, wontByte, doByte, dontByte:
				f.pendingCmd = b
				f.state = stateGotCommand
			case sbByte:
				f.state = stateInSubneg
			default:
				// Single-byte command (NOP, AYT, etc.) with no option
				// byte to follow; nothing to reply.
				f.state = stateNormal
			}

		case stateGotCommand:
			f.reply(f.pendingCmd, b)
			f.state = stateNormal

		case stateInSubneg:
			if b == iacByte {
				f.state = stateSubnegIAC
			}
			// else: discard subnegotiation payload bytes.

		case stateSubnegIAC:
			if b == seByte {
				f.state = stateNormal
			} else {
				// Not a real terminator; stay in the subnegotiation.
				f.state = stateInSubneg
			}
		}
	}

	return plain
}

// reply refuses whatever option the remote proposed: WILL -> DONT,
// DO -> WONT. This client never opts into any Telnet option.
func (f *iacFilter) reply(cmd, option byte) {
	var response byte
	switch cmd {
	case willByte:
		response = dontByte
	case doByte:
		response = wontByte
	default:
		// WONT/DONT from the remote require no reply.
		return
	}
	_, _ = f.conn.Write([]byte{iacByte, response, option})
}
