package herdr

import (
	"bufio"
	"encoding/json"
	"io"
	"net"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

// fakeHerdr is a Unix-socket server speaking herdr's newline-delimited JSON,
// with frames shaped from docs/next/api/herdr-api.schema.json (protocol 22).
type fakeHerdr struct {
	t      *testing.T
	path   string
	ln     net.Listener
	mu     sync.Mutex
	calls  []string                     // every method received, in order
	result map[string]any               // method -> result object
	cur    chan []byte                  // lines for the newest subscriber
	subs   chan []map[string]any        // the subscriptions each events.subscribe carried
	errFor map[string]map[string]string // method -> error body
}

func newFake(t *testing.T) *fakeHerdr {
	t.Helper()
	dir, err := os.MkdirTemp("", "hd")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })
	f := &fakeHerdr{t: t, path: filepath.Join(dir, "herdr.sock"), result: map[string]any{},
		subs: make(chan []map[string]any, 16), errFor: map[string]map[string]string{}}
	f.ln, err = net.Listen("unix", f.path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { f.ln.Close() })
	go f.accept()
	return f
}

func (f *fakeHerdr) accept() {
	for {
		c, err := f.ln.Accept()
		if err != nil {
			return
		}
		go f.serve(c)
	}
}

func (f *fakeHerdr) serve(c net.Conn) {
	defer c.Close()
	sc := bufio.NewScanner(c)
	sc.Buffer(make([]byte, 1<<20), 1<<22)
	for sc.Scan() {
		var req struct {
			ID     string         `json:"id"`
			Method string         `json:"method"`
			Params map[string]any `json:"params"`
		}
		if json.Unmarshal(sc.Bytes(), &req) != nil {
			return
		}
		f.mu.Lock()
		f.calls = append(f.calls, req.Method)
		res, hasRes := f.result[req.Method]
		eb := f.errFor[req.Method]
		f.mu.Unlock()
		var resp map[string]any
		switch {
		case eb != nil:
			resp = map[string]any{"id": req.ID, "error": eb}
		case req.Method == "events.subscribe":
			resp = map[string]any{"id": req.ID, "result": map[string]any{"type": "subscription_started"}}
		case hasRes:
			resp = map[string]any{"id": req.ID, "result": res}
		default:
			resp = map[string]any{"id": req.ID, "error": map[string]string{"code": "unknown_method", "message": req.Method}}
		}
		b, _ := json.Marshal(resp)
		c.Write(append(b, '\n'))
		if req.Method != "events.subscribe" || eb != nil {
			return // herdr v0.9.5 closes a connection after answering one request
		}
		if req.Method == "events.subscribe" && eb == nil {
			subs, _ := req.Params["subscriptions"].([]any)
			var list []map[string]any
			for _, s := range subs {
				list = append(list, s.(map[string]any))
			}
			ch := make(chan []byte, 16)
			f.mu.Lock()
			f.cur = ch
			f.mu.Unlock()
			f.subs <- list
			done := make(chan struct{})
			go func() { io.Copy(io.Discard, c); close(done) }()
			for {
				select {
				case line := <-ch:
					c.Write(append(line, '\n'))
				case <-done:
					return
				}
			}
		}
	}
}

func (f *fakeHerdr) methods() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.calls...)
}

func (f *fakeHerdr) set(method string, result any) {
	f.mu.Lock()
	f.result[method] = result
	f.mu.Unlock()
}

// send pushes one event line to the newest subscriber.
func (f *fakeHerdr) send(line string) {
	f.mu.Lock()
	ch := f.cur
	f.mu.Unlock()
	ch <- []byte(line)
}
