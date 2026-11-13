// Package termhost keeps the floating terminal's programs running while
// Gocast quits, crashes or updates: a small process of its own, the same
// executable started as `Gocast --terminal-host <data folder>`, runs each
// tab's terminal, and Gocast shows them over a Unix socket in the data
// folder, with MyGo's Mirror frames (docs/dev-notes.md, "The terminal
// host").
//
// The host starts with the first terminal, lives only in memory, and
// exits 10 seconds after its last terminal ends, or once Gocast.app is
// gone (uninstalled).
package termhost

import (
	"bufio"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
)

// Proto is the version of the protocol. Version 1's messages stay as they
// are: later versions only add messages and capabilities, so that a newer
// Gocast keeps talking to an older host still running its terminals, and
// an older Gocast to a newer host.
const Proto = 1

// The capabilities of this host, in its hello: what a client may use
// beyond version 1's core (create, list, attach, input, resize, close).
const (
	CapPaste   = "paste"   // msgPaste: pasted formats, for the host's PasteData
	CapClear   = "clear"   // msgClear: the user's clear
	CapVisible = "visible" // msgVisible: frames slower while unseen
	CapTheme   = "theme"   // msgTheme: the colors the client shows
)

var caps = []string{CapPaste, CapClear, CapVisible, CapTheme}

// The messages: a type byte, the payload's length (uint32, big-endian),
// the payload. Requests and replies are JSON; input and frames are bytes.
const (
	// Client to host, on any connection, first.
	msgHello byte = 1
	// Client to host, on the control connection, each answered by a
	// msgReply.
	msgList      byte = 2
	msgCreate    byte = 3
	msgClose     byte = 4
	msgCloseAll  byte = 5
	msgSetMeta   byte = 6
	msgSetActive byte = 7
	// msgAttach makes the connection a session's stream: answered by a
	// msgReply, then frames and, once the program ends, msgExit.
	msgAttach byte = 8

	// Client to host, on a stream.
	msgInput   byte = 16
	msgResize  byte = 17
	msgTheme   byte = 18
	msgPaste   byte = 19
	msgClear   byte = 20
	msgVisible byte = 21

	// Host to client.
	msgReply byte = 64
	msgFrame byte = 65 // uint16 cols, uint16 rows, the frame's bytes
	msgExit  byte = 66 // JSON exitMsg
)

// maxMessage is the largest message read: a frame of a full scrollback
// with images, or a paste of an image.
const maxMessage = 256 << 20

type hello struct {
	Proto int      `json:"proto"`
	Build string   `json:"build,omitempty"`
	PID   int      `json:"pid,omitempty"`
	Caps  []string `json:"caps,omitempty"`
}

// reply answers a request: Error when it failed, else the request's
// result in the other fields.
type reply struct {
	Error    string    `json:"error,omitempty"`
	Hello    *hello    `json:"hello,omitempty"`
	ID       string    `json:"id,omitempty"`
	Sessions []Session `json:"sessions,omitempty"`
	Active   string    `json:"active,omitempty"`
}

// Session is a terminal the host runs, as list returns it.
type Session struct {
	ID string `json:"id"`
	// Meta is what the client said of the session, kept for it as is.
	Meta json.RawMessage `json:"meta,omitempty"`
	// Title is what the program last set (OSC 2).
	Title string `json:"title,omitempty"`
	// Done tells that the program once set CreateRequest.DoneTitle: a
	// preset's command ended, its shell going on.
	Done bool `json:"done,omitempty"`
}

// CreateRequest starts a terminal.
type CreateRequest struct {
	Argv []string        `json:"argv,omitempty"` // nil: the user's shell
	Dir  string          `json:"dir,omitempty"`
	Env  []string        `json:"env,omitempty"`
	Meta json.RawMessage `json:"meta,omitempty"`
	// DoneTitle is the title that makes Session.Done, for good.
	DoneTitle string `json:"doneTitle,omitempty"`
	// Prefill shows before the program's output: what a terminal that
	// was cut off showed, at Cols×Rows, which the program then starts at.
	Prefill []byte `json:"prefill,omitempty"`
	// Cols and Rows, with the size of a cell in pixels, start the program
	// at once; zero, it starts at the first stream's size.
	Cols  int `json:"cols,omitempty"`
	Rows  int `json:"rows,omitempty"`
	CellW int `json:"cellW,omitempty"`
	CellH int `json:"cellH,omitempty"`
}

type idMsg struct {
	ID   string          `json:"id"`
	Meta json.RawMessage `json:"meta,omitempty"`
}

type sizeMsg struct {
	Cols, Rows, CellW, CellH int
}

type exitMsg struct {
	Code int `json:"code"`
}

// themeMsg is a terminal.Theme in RGBA.
type themeMsg struct {
	Foreground, Background, Cursor, CursorText [4]uint8
	Palette                                    [16][4]uint8
}

// pasteMsg is a paste's formats, as MIME types or MyGo's names.
type pasteMsg struct {
	Formats []string `json:"formats"`
	Data    [][]byte `json:"data"`
}

// writeMsg writes a message in one write.
func writeMsg(w io.Writer, typ byte, payload []byte) error {
	if len(payload) > maxMessage {
		return fmt.Errorf("termhost: message of %d bytes", len(payload))
	}
	b := make([]byte, 5+len(payload))
	b[0] = typ
	binary.BigEndian.PutUint32(b[1:5], uint32(len(payload)))
	copy(b[5:], payload)
	_, err := w.Write(b)
	return err
}

func writeJSON(w io.Writer, typ byte, v any) error {
	b, err := json.Marshal(v)
	if err != nil {
		return err
	}
	return writeMsg(w, typ, b)
}

// readMsg reads a message.
func readMsg(r *bufio.Reader) (typ byte, payload []byte, err error) {
	var h [5]byte
	if _, err := io.ReadFull(r, h[:]); err != nil {
		return 0, nil, err
	}
	n := binary.BigEndian.Uint32(h[1:])
	if n > maxMessage {
		return 0, nil, fmt.Errorf("termhost: message of %d bytes", n)
	}
	payload = make([]byte, n)
	if _, err := io.ReadFull(r, payload); err != nil {
		if errors.Is(err, io.EOF) {
			err = io.ErrUnexpectedEOF
		}
		return 0, nil, err
	}
	return h[0], payload, nil
}

// frameMsg is a frame with the size the host's terminal had as it made it.
func frameMsg(cols, rows int, frame []byte) []byte {
	b := make([]byte, 4+len(frame))
	binary.BigEndian.PutUint16(b[0:2], uint16(cols))
	binary.BigEndian.PutUint16(b[2:4], uint16(rows))
	copy(b[4:], frame)
	return b
}

func parseFrame(p []byte) (cols, rows int, frame []byte, err error) {
	if len(p) < 4 {
		return 0, 0, nil, errors.New("termhost: short frame")
	}
	return int(binary.BigEndian.Uint16(p[0:2])), int(binary.BigEndian.Uint16(p[2:4])), p[4:], nil
}
