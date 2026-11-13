package termhost

import (
	"bufio"
	"encoding/json"
	"errors"
	"io"
	"net"
	"slices"
	"sync"
	"time"

	"github.com/egoist/mygo/plugins/terminal"
	"github.com/egoist/mygo/transfer"
)

// requestTimeout bounds a request to a host that stopped answering; a
// CloseAll waits for the programs to end.
const (
	requestTimeout  = 5 * time.Second
	closeAllTimeout = closeGrace + 5*time.Second
)

// ErrLost is the error of a request to a host that went away.
var ErrLost = errors.New("termhost: the host went away")

// Client is a connection to a host, for requests; each terminal it shows
// takes a Stream of its own.
type Client struct {
	dial  func() (net.Conn, error)
	hello hello

	mu   sync.Mutex
	conn net.Conn
	r    *bufio.Reader
	lost bool
}

// NewClient connects to a host with dial, which Attach calls again for
// each stream.
func NewClient(dial func() (net.Conn, error)) (*Client, error) {
	c := &Client{dial: dial}
	conn, r, h, err := c.connect()
	if err != nil {
		return nil, err
	}
	c.conn, c.r, c.hello = conn, r, h
	return c, nil
}

// connect dials the host and says hello.
func (c *Client) connect() (net.Conn, *bufio.Reader, hello, error) {
	conn, err := c.dial()
	if err != nil {
		return nil, nil, hello{}, err
	}
	conn.SetDeadline(time.Now().Add(requestTimeout))
	r := bufio.NewReader(conn)
	var rep reply
	err = writeJSON(conn, msgHello, hello{Proto: Proto})
	if err == nil {
		rep, err = readReply(r)
	}
	if err == nil && rep.Hello == nil {
		err = errors.New("termhost: no hello")
	}
	if err != nil {
		conn.Close()
		return nil, nil, hello{}, err
	}
	conn.SetDeadline(time.Time{})
	return conn, r, *rep.Hello, nil
}

func readReply(r *bufio.Reader) (reply, error) {
	typ, p, err := readMsg(r)
	if err != nil {
		return reply{}, err
	}
	if typ != msgReply {
		return reply{}, errors.New("termhost: not a reply")
	}
	var rep reply
	if err := json.Unmarshal(p, &rep); err != nil {
		return reply{}, err
	}
	if rep.Error != "" {
		return rep, errors.New(rep.Error)
	}
	return rep, nil
}

// PID is the host's process ID.
func (c *Client) PID() int { return c.hello.PID }

// Has reports whether the host has the capability (CapPaste, ...).
func (c *Client) Has(capability string) bool { return slices.Contains(c.hello.Caps, capability) }

// Lost reports whether a request found the host gone.
func (c *Client) Lost() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.lost
}

// Close closes the client's connection: the host's terminals go on.
func (c *Client) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.lost = true
	return c.conn.Close()
}

// request sends a request and reads its reply. A reply's error is the
// error; ErrLost when the connection broke.
func (c *Client) request(typ byte, v any, timeout time.Duration) (reply, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.lost {
		return reply{}, ErrLost
	}
	c.conn.SetDeadline(time.Now().Add(timeout))
	defer c.conn.SetDeadline(time.Time{})
	if err := writeJSON(c.conn, typ, v); err != nil {
		c.lost = true
		return reply{}, ErrLost
	}
	typ, p, err := readMsg(c.r)
	if err != nil || typ != msgReply {
		c.lost = true
		return reply{}, ErrLost
	}
	var rep reply
	if err := json.Unmarshal(p, &rep); err != nil {
		return reply{}, err
	}
	if rep.Error != "" {
		return rep, errors.New(rep.Error)
	}
	return rep, nil
}

// List returns the host's terminals, in the order they were made, and
// the ID SetActive last set.
func (c *Client) List() (sessions []Session, active string, err error) {
	rep, err := c.request(msgList, struct{}{}, requestTimeout)
	return rep.Sessions, rep.Active, err
}

// Create starts a terminal and returns its ID.
func (c *Client) Create(req CreateRequest) (string, error) {
	rep, err := c.request(msgCreate, req, requestTimeout)
	return rep.ID, err
}

// CloseTerminal ends a terminal: its program is hung up.
func (c *Client) CloseTerminal(id string) error {
	_, err := c.request(msgClose, idMsg{ID: id}, requestTimeout)
	return err
}

// CloseAll ends every terminal, waits for their programs to end, and has
// the host exit.
func (c *Client) CloseAll() error {
	_, err := c.request(msgCloseAll, struct{}{}, closeAllTimeout)
	return err
}

// SetMeta keeps meta with the terminal, for List to return.
func (c *Client) SetMeta(id string, meta json.RawMessage) error {
	_, err := c.request(msgSetMeta, idMsg{ID: id, Meta: meta}, requestTimeout)
	return err
}

// SetActive keeps the terminal showing, for List to return.
func (c *Client) SetActive(id string) error {
	_, err := c.request(msgSetActive, idMsg{ID: id}, requestTimeout)
	return err
}

// Attach opens a stream of the terminal: its frames, from the first,
// which shows all it holds, and its input.
func (c *Client) Attach(id string) (*Stream, error) {
	conn, r, h, err := c.connect()
	if err != nil {
		return nil, ErrLost
	}
	conn.SetDeadline(time.Now().Add(requestTimeout))
	if err := writeJSON(conn, msgAttach, idMsg{ID: id}); err != nil {
		conn.Close()
		return nil, ErrLost
	}
	if _, err := readReply(r); err != nil {
		conn.Close()
		return nil, err
	}
	conn.SetDeadline(time.Time{})
	return &Stream{conn: conn, r: r, caps: h.Caps, ready: make(chan struct{})}, nil
}

// Stream is a terminal of the host as a terminal.Options.Conn, which
// ResizeByConn: its frames show the host's terminal, at the host's size,
// which Bind's resize gives the client's terminal before each frame of a
// new size.
type Stream struct {
	conn net.Conn
	r    *bufio.Reader
	caps []string
	wmu  sync.Mutex

	ready     chan struct{}
	readyOnce sync.Once
	resize    func(cols, rows int)

	// Read's: the rest of the frame being read, and the size of the
	// frames.
	buf        []byte
	cols, rows int

	mu     sync.Mutex
	exited bool // the program ended, as the host said
	code   int
	closed bool // Close
	lost   bool // the host went away
}

// Bind gives the stream what resizes the terminal it feeds; Read waits
// for it.
func (s *Stream) Bind(resize func(cols, rows int)) {
	s.resize = resize
	s.readyOnce.Do(func() { close(s.ready) })
}

// Read reads the frames.
func (s *Stream) Read(p []byte) (int, error) {
	<-s.ready
	for len(s.buf) == 0 {
		typ, msg, err := readMsg(s.r)
		if err != nil {
			s.mu.Lock()
			s.lost = !s.closed && !s.exited
			s.mu.Unlock()
			return 0, io.EOF
		}
		switch typ {
		case msgFrame:
			cols, rows, frame, err := parseFrame(msg)
			if err != nil {
				return 0, io.EOF
			}
			// The terminal takes the host's size before the frame, which
			// the host laid out at it: every byte before it was written
			// (the terminal reads again once it wrote what it read).
			if (cols != s.cols || rows != s.rows) && s.resize != nil {
				s.cols, s.rows = cols, rows
				s.resize(cols, rows)
			}
			s.buf = frame
		case msgExit:
			var e exitMsg
			_ = json.Unmarshal(msg, &e)
			s.mu.Lock()
			s.exited, s.code = true, e.Code
			s.mu.Unlock()
			return 0, io.EOF
		}
	}
	n := copy(p, s.buf)
	s.buf = s.buf[n:]
	return n, nil
}

func (s *Stream) send(typ byte, payload []byte) error {
	s.wmu.Lock()
	defer s.wmu.Unlock()
	return writeMsg(s.conn, typ, payload)
}

func (s *Stream) sendJSON(typ byte, v any) error {
	b, err := json.Marshal(v)
	if err != nil {
		return err
	}
	return s.send(typ, b)
}

// Write sends what is typed.
func (s *Stream) Write(p []byte) (int, error) {
	if err := s.send(msgInput, p); err != nil {
		return 0, err
	}
	return len(p), nil
}

// Close closes the stream: the terminal goes on in the host.
func (s *Stream) Close() error {
	s.mu.Lock()
	s.closed = true
	s.mu.Unlock()
	s.readyOnce.Do(func() { close(s.ready) })
	return s.conn.Close()
}

// ResizePixels asks the host for the size the view fits.
func (s *Stream) ResizePixels(cols, rows, cellWidth, cellHeight int) error {
	return s.sendJSON(msgResize, sizeMsg{Cols: cols, Rows: rows, CellW: cellWidth, CellH: cellHeight})
}

// UseTheme tells the host the colors the view shows, which programs ask.
func (s *Stream) UseTheme(theme *terminal.Theme) error {
	if theme == nil || !slices.Contains(s.caps, CapTheme) {
		return nil
	}
	return s.sendJSON(msgTheme, themeOf(theme))
}

// Paste pastes d in the host's terminal, as its PasteData: every format,
// for a program that reads them itself (OSC 5522); only the text when they
// would carry more than 32 MB. It reports false when the host cannot.
func (s *Stream) Paste(d transfer.Data) bool {
	if !slices.Contains(s.caps, CapPaste) {
		return false
	}
	var pm pasteMsg
	size := 0
	for _, f := range d.Formats() {
		b, err := d.Read(f)
		if err != nil {
			continue
		}
		pm.Formats = append(pm.Formats, string(f))
		pm.Data = append(pm.Data, b)
		size += len(b)
	}
	if size > maxPaste {
		pm = pasteMsg{}
		if b, err := d.Read(transfer.Text); err == nil && len(b) <= maxPaste {
			pm.Formats, pm.Data = []string{string(transfer.Text)}, [][]byte{b}
		}
	}
	return s.sendJSON(msgPaste, pm) == nil
}

// Clear has the host's terminal clear, as the user asked: the next frame
// shows it. It reports false when the host cannot.
func (s *Stream) Clear() bool {
	return slices.Contains(s.caps, CapClear) && s.send(msgClear, nil) == nil
}

// SetVisible tells the host whether the terminal shows, for it to look
// for changes less often while it doesn't.
func (s *Stream) SetVisible(visible bool) {
	if !slices.Contains(s.caps, CapVisible) {
		return
	}
	b := byte(0)
	if visible {
		b = 1
	}
	_ = s.send(msgVisible, []byte{b})
}

// Ended tells how the stream ended, once Read returned io.EOF: the
// program exited with code; or else, lost, the host went away, unless
// Close closed it.
func (s *Stream) Ended() (exited bool, code int, lost bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.exited, s.code, s.lost
}
