package herdr

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"strconv"
	"time"
)

// allowedMethods is the whole of what this adapter may ask herdr. It is the
// read-only guarantee: send refuses anything else, so a future edit cannot
// type into a pane by accident. TestClientOnlyIssuesAllowlistedMethods holds
// the line from the other side.
var allowedMethods = map[string]bool{
	"ping":              true,
	"agent.list":        true,
	"agent.explain":     true,
	"pane.process_info": true,
	"events.subscribe":  true,
}

// maxLine bounds one frame; a session snapshot is the largest and stays far
// below this.
const maxLine = 4 << 20

// errDisallowed is returned for a method outside allowedMethods.
var errDisallowed = errors.New("herdr: method not on the read-only allowlist")

// conn is one newline-delimited JSON connection (herdr: docs/socket-api,
// "Socket transport"). Requests are {"id","method","params"}; responses are
// {"id","result"} or {"id","error":{"code","message"}}.
type conn struct {
	c       net.Conn
	r       *bufio.Reader
	n       int
	timeout time.Duration
}

func dial(path string, timeout time.Duration) (*conn, error) {
	c, err := net.DialTimeout("unix", path, timeout)
	if err != nil {
		return nil, err
	}
	return &conn{c: c, r: bufio.NewReaderSize(c, 64<<10), timeout: timeout}, nil
}

func (c *conn) close() { _ = c.c.Close() }

// send writes one request line and returns its id.
func (c *conn) send(method string, params any) (string, error) {
	if !allowedMethods[method] {
		return "", fmt.Errorf("%w: %q", errDisallowed, method)
	}
	c.n++
	id := "fleet_" + strconv.Itoa(c.n)
	b, err := json.Marshal(map[string]any{"id": id, "method": method, "params": params})
	if err != nil {
		return "", err
	}
	_ = c.c.SetWriteDeadline(time.Now().Add(c.timeout))
	_, err = c.c.Write(append(b, '\n'))
	return id, err
}

// line reads one frame. A zero deadline waits indefinitely (event streams).
func (c *conn) line(deadline time.Time) ([]byte, error) {
	_ = c.c.SetReadDeadline(deadline)
	var out []byte
	for {
		part, err := c.r.ReadSlice('\n')
		out = append(out, part...)
		if len(out) > maxLine {
			return nil, errors.New("herdr: frame too large")
		}
		if err == bufio.ErrBufferFull {
			continue
		}
		return out, err
	}
}

type frame struct {
	ID     string          `json:"id"`
	Result json.RawMessage `json:"result"`
	Error  *struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
	Event string          `json:"event"`
	Data  json.RawMessage `json:"data"`
}

// rpcError is an error response from herdr.
type rpcError struct{ Code, Message string }

func (e *rpcError) Error() string { return "herdr: " + e.Code + ": " + e.Message }

// call sends one request and waits for the response with its id, discarding
// any event frames that arrive first. The result is decoded into out (nil to
// ignore it).
func (c *conn) call(method string, params, out any) error {
	id, err := c.send(method, params)
	if err != nil {
		return err
	}
	deadline := time.Now().Add(c.timeout)
	for {
		b, err := c.line(deadline)
		if err != nil {
			return err
		}
		var f frame
		if json.Unmarshal(b, &f) != nil || f.ID != id {
			continue
		}
		if f.Error != nil {
			return &rpcError{f.Error.Code, f.Error.Message}
		}
		if out == nil {
			return nil
		}
		return json.Unmarshal(f.Result, out)
	}
}

// callOnce makes one request on its own connection. herdr answers one request
// per connection and then closes it (seen live on v0.9.5: a second write fails
// with a broken pipe), so every request but the event stream's gets a fresh one.
func callOnce(path string, timeout time.Duration, method string, params, out any) error {
	c, err := dial(path, timeout)
	if err != nil {
		return err
	}
	defer c.close()
	return c.call(method, params, out)
}
