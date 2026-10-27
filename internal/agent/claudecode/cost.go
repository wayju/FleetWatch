package claudecode

// The cost accumulator. It is independent of the display tailer (which starts
// near the end of a transcript): every file of a session, main and subagents,
// is read once from byte 0 in the background, then only the bytes appended
// since. The poll loop never waits on it; it asks for a snapshot of whatever
// has been counted so far and a partial flag.
//
// Claude Code writes one JSONL line per content block, each repeating the
// message's usage, so lines are deduplicated by message id (output_tokens
// grows over a message's lines, so the last line is the one that counts). The lines of one
// message are adjacent apart from replays of earlier history, which measured
// at most 143 lines back on this host's transcripts, so each file remembers
// the last costIDRing ids rather than every id it has seen.

import (
	"github.com/wayju/fleetwatch/internal/board"
	"github.com/wayju/fleetwatch/internal/fsafe"
	"github.com/wayju/fleetwatch/internal/models"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"
)

const (
	costWorkers = 2 // files read at once; keeps the first pass off the poll's back
	costIDRing  = 256
	costRefresh = 4 * time.Second // a session's file list is re-statted this often
	costKeep    = 2 * time.Hour   // an unrequested file's state is dropped after this
	costQueue   = 4096
)

// idRing remembers the most recent message ids as 64-bit hashes: a file's ring
// costs about 6 KiB instead of the 20 KiB its strings would, which matters
// when a busy session has hundreds of subagent transcripts.
type idRing struct {
	ids  []uint64
	set  map[uint64]struct{}
	next int
}

func hashID(id string) uint64 {
	h := uint64(14695981039346656037) // FNV-1a
	for i := 0; i < len(id); i++ {
		h = (h ^ uint64(id[i])) * 1099511628211
	}
	return h | 1 // never 0, which marks an empty slot
}

// seen reports whether id is already remembered, remembering it if not.
func (r *idRing) seen(id string) bool {
	h := hashID(id)
	if _, ok := r.set[h]; ok {
		return true
	}
	if r.set == nil {
		r.set = make(map[uint64]struct{}, costIDRing)
		r.ids = make([]uint64, costIDRing)
	}
	if old := r.ids[r.next]; old != 0 {
		delete(r.set, old)
	}
	r.ids[r.next] = h
	r.set[h] = struct{}{}
	r.next = (r.next + 1) % costIDRing
	return false
}

type costFile struct {
	path     string
	offset   int64 // bytes folded in, always at a line start
	sizeSeen int64 // file size when the last scan began
	ino      uint64
	scanning bool
	done     bool // the first full read has finished
	ring     idRing
	last     lastMsg // the newest counted message, whose later lines may still raise its usage
	models   map[string]*board.ModelCost
	wanted   time.Time
}

// lastMsg is the message most recently counted. Claude Code writes the first
// line of a message while it is still streaming, so output_tokens grows over
// the message's lines (the final count is on the last); the other fields
// repeat unchanged.
type lastMsg struct {
	id     string
	model  string
	tokens board.CostTokens
	usd    float64
}

type costSession struct {
	files   []*costFile
	checked time.Time
}

type CostAccumulator struct {
	prices   *PriceTable
	models   *models.Table // classifies by_model rows; nil leaves them unclassed
	mu       sync.Mutex
	files    map[string]*costFile
	sessions map[string]*costSession // main transcript path
	queue    chan *costFile
}

func NewCostAccumulator(prices *PriceTable) *CostAccumulator {
	a := &CostAccumulator{
		prices: prices, files: map[string]*costFile{}, sessions: map[string]*costSession{},
		queue: make(chan *costFile, costQueue),
	}
	for i := 0; i < costWorkers; i++ {
		go func() {
			for cf := range a.queue {
				a.scan(cf)
			}
		}()
	}
	return a
}

// Session returns the cost counted so far for the session whose main
// transcript is mainPath, and queues reads for any of its files that are new
// or have grown. It stats at most once per costRefresh.
func (a *CostAccumulator) Session(mainPath string, now time.Time) board.SessionCost {
	a.mu.Lock()
	defer a.mu.Unlock()
	s := a.sessions[mainPath]
	if s == nil || now.Sub(s.checked) >= costRefresh {
		if s == nil {
			s = &costSession{}
			a.sessions[mainPath] = s
		}
		s.checked = now
		s.files = s.files[:0]
		for _, p := range sessionFiles(mainPath) {
			if cf := a.track(p, now); cf != nil {
				s.files = append(s.files, cf)
			}
		}
	}
	return a.sum(s.files)
}

// sessionFiles lists the main transcript and every subagent transcript.
func sessionFiles(mainPath string) []string {
	subs, _ := filepath.Glob(filepath.Join(strings.TrimSuffix(mainPath, ".jsonl"), "subagents", "*.jsonl"))
	return append([]string{mainPath}, subs...)
}

// track stats one file and queues a read when it is new or changed. Caller
// holds mu.
func (a *CostAccumulator) track(path string, now time.Time) *costFile {
	st, err := os.Lstat(path)
	if err != nil || !st.Mode().IsRegular() {
		return a.files[path] // keep what a vanished file already contributed
	}
	cf := a.files[path]
	if cf == nil {
		cf = &costFile{path: path, models: map[string]*board.ModelCost{}}
		a.files[path] = cf
	}
	cf.wanted = now
	if !cf.scanning && (!cf.done || st.Size() != cf.sizeSeen || inode(st) != cf.ino) {
		cf.scanning = true
		select {
		case a.queue <- cf:
		default:
			cf.scanning = false // queue full: try again next refresh
		}
	}
	return cf
}

func inode(st os.FileInfo) uint64 {
	if s, ok := st.Sys().(*syscall.Stat_t); ok {
		return s.Ino
	}
	return 0
}

// sum folds the files' per-model counts into one board.SessionCost. Caller holds mu.
func (a *CostAccumulator) sum(files []*costFile) board.SessionCost {
	out := board.EmptyCost(a.prices.AsOf)
	rows := make([][]board.ModelCost, 0, len(files))
	for _, cf := range files {
		if !cf.done {
			out.Partial = true
		}
		set := make([]board.ModelCost, 0, len(cf.models))
		for _, m := range cf.models {
			set = append(set, *m)
		}
		rows = append(rows, set)
	}
	out.ByModel = board.MergeModels(rows...)
	for i, m := range out.ByModel {
		out.ByModel[i].Class = a.models.Class(m.Model)
		out.USD += m.USD
		out.Tokens.Add(m.Tokens)
	}
	return out
}

// scan reads the unread bytes of one file and folds them in. Only this
// goroutine touches the file's offset, ring and counters' writers; readers see
// them under mu.
func (a *CostAccumulator) scan(cf *costFile) {
	f, st, err := fsafe.OpenRegular(cf.path)
	if err != nil {
		a.finish(cf, 0, 0)
		return
	}
	defer f.Close()
	a.mu.Lock()
	if ino := inode(st); st.Size() < cf.offset || (cf.ino != 0 && ino != cf.ino) {
		// Truncated or replaced: count the new file from the start.
		cf.offset, cf.ring, cf.last, cf.models, cf.done = 0, idRing{}, lastMsg{}, map[string]*board.ModelCost{}, false
	}
	cf.ino = inode(st)
	start, size := cf.offset, st.Size()
	a.mu.Unlock()
	if size <= start {
		a.finish(cf, size, 0)
		return
	}
	if _, err := f.Seek(start, io.SeekStart); err != nil {
		a.finish(cf, size, 0)
		return
	}
	consumed, _, _ := scanLines(io.LimitReader(f, size-start), costMaxLine, func(line []byte) {
		if rec, ok := parseUsage(line); ok {
			a.apply(cf, rec)
		}
	})
	a.finish(cf, size, consumed)
}

func (a *CostAccumulator) finish(cf *costFile, size, consumed int64) {
	a.mu.Lock()
	cf.offset += consumed
	cf.sizeSeen, cf.done, cf.scanning = size, true, false
	a.mu.Unlock()
}

// apply folds one usage record into the file's counts. A repeat of the newest
// message replaces its usage with the field-wise maximum; a repeat of an
// earlier one (a replayed history) is ignored.
func (a *CostAccumulator) apply(cf *costFile, rec usageRecord) {
	a.mu.Lock()
	defer a.mu.Unlock()
	name := rec.Model
	if name == "" {
		name = "unknown"
	}
	if rec.ID != "" && rec.ID == cf.last.id && name == cf.last.model {
		cf.raise(a.prices, name, rec.Tokens)
		return
	}
	if rec.ID != "" && cf.ring.seen(rec.ID) {
		return
	}
	m := cf.models[name]
	if m == nil {
		m = &board.ModelCost{Model: name}
		cf.models[name] = m
	}
	usd, ok := a.prices.price(rec.Model, rec.Tokens)
	m.USD += usd
	m.Unpriced = !ok
	m.Messages++
	m.Tokens.Add(rec.Tokens)
	cf.last = lastMsg{id: rec.ID, model: name, tokens: rec.Tokens, usd: usd}
}

// raise lifts the newest message to the larger of each token count and
// reprices it, since a tier can depend on the totals.
func (cf *costFile) raise(pt *PriceTable, name string, t board.CostTokens) {
	old := cf.last.tokens
	up := board.CostTokens{
		Input: max(old.Input, t.Input), CacheWrite5m: max(old.CacheWrite5m, t.CacheWrite5m),
		CacheWrite1h: max(old.CacheWrite1h, t.CacheWrite1h), CacheRead: max(old.CacheRead, t.CacheRead),
		Output: max(old.Output, t.Output),
	}
	if up == old {
		return
	}
	m := cf.models[name]
	usd, _ := pt.price(name, up)
	m.USD += usd - cf.last.usd
	m.Tokens.Input += up.Input - old.Input
	m.Tokens.CacheWrite5m += up.CacheWrite5m - old.CacheWrite5m
	m.Tokens.CacheWrite1h += up.CacheWrite1h - old.CacheWrite1h
	m.Tokens.CacheRead += up.CacheRead - old.CacheRead
	m.Tokens.Output += up.Output - old.Output
	cf.last.tokens, cf.last.usd = up, usd
}

// Prune drops state for files no session has asked about for costKeep.
func (a *CostAccumulator) Prune(now time.Time) {
	a.mu.Lock()
	defer a.mu.Unlock()
	for p, cf := range a.files {
		if !cf.scanning && now.Sub(cf.wanted) > costKeep {
			delete(a.files, p)
		}
	}
	for p, s := range a.sessions {
		if now.Sub(s.checked) > costKeep {
			delete(a.sessions, p)
		}
	}
}

// AsOf is the date of the price table in use.
func (a *CostAccumulator) AsOf() string { return a.prices.AsOf }

// WaitIdle blocks until every file the accumulator was asked to read has had
// its full first read, or the timeout passes. Tests use it to make the
// background reads deterministic; the poll loop never waits.
func (a *CostAccumulator) WaitIdle(timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		a.mu.Lock()
		idle := true
		for _, cf := range a.files {
			if cf.scanning || !cf.done {
				idle = false
				break
			}
		}
		a.mu.Unlock()
		if idle {
			return true
		}
		time.Sleep(5 * time.Millisecond)
	}
	return false
}
