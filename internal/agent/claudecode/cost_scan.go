package claudecode

// The streaming half of the cost accumulator: reads whole JSONL lines from a
// reader with a hard line cap, and turns an assistant line into one usage
// record.

import (
	"bufio"
	"bytes"
	"encoding/json"
	"github.com/wayju/fleetwatch/internal/board"
	"io"
)

const (
	costMaxLine = 8 << 20 // a longer line is skipped, never buffered
	costBufSize = 256 << 10
)

var usageKey = []byte(`"usage"`)

// scanLines calls fn for each complete line (newline-terminated) from r and
// returns how many bytes those lines span. An unterminated tail is not
// consumed, so the caller resumes at its start once the writer finishes it.
// A line over max is skipped and counted. fn must not keep the slice.
func scanLines(r io.Reader, max int, fn func([]byte)) (consumed int64, skipped int, err error) {
	br := bufio.NewReaderSize(r, costBufSize)
	var buf []byte
	var pending int64 // bytes of the line being read, newline included
	skipping := false
	for {
		chunk, rerr := br.ReadSlice('\n')
		pending += int64(len(chunk))
		complete := len(chunk) > 0 && chunk[len(chunk)-1] == '\n'
		switch {
		case skipping:
		case len(buf)+len(chunk) > max:
			buf, skipping = nil, true
		case complete && len(buf) == 0:
			fn(chunk) // the whole line sits in the reader's buffer
		default:
			buf = append(buf, chunk...)
			if complete {
				fn(buf)
				buf = buf[:0]
			}
		}
		if complete {
			if skipping {
				skipped++
				skipping = false
			}
			consumed += pending
			pending = 0
		}
		if rerr == bufio.ErrBufferFull {
			continue
		}
		if rerr != nil {
			if rerr == io.EOF {
				rerr = nil
			}
			return consumed, skipped, rerr
		}
	}
}

// usageRecord is one assistant message's usage. ID is empty when the line has
// none, which is then never deduplicated.
type usageRecord struct {
	ID     string
	Model  string
	Tokens board.CostTokens
}

type costUsage struct {
	Input         int64 `json:"input_tokens"`
	CacheCreation int64 `json:"cache_creation_input_tokens"`
	CacheRead     int64 `json:"cache_read_input_tokens"`
	Output        int64 `json:"output_tokens"`
	Split         *struct {
		M5 int64 `json:"ephemeral_5m_input_tokens"`
		H1 int64 `json:"ephemeral_1h_input_tokens"`
	} `json:"cache_creation"`
}

type costLine struct {
	Message *struct {
		ID    string     `json:"id"`
		Model string     `json:"model"`
		Usage *costUsage `json:"usage"`
	} `json:"message"`
}

// parseUsage extracts the usage of one transcript line. Lines without a
// message usage (most of them) are rejected by a byte search before any JSON
// is decoded.
func parseUsage(line []byte) (usageRecord, bool) {
	if !bytes.Contains(line, usageKey) {
		return usageRecord{}, false
	}
	var l costLine
	if json.Unmarshal(line, &l) != nil || l.Message == nil || l.Message.Usage == nil {
		return usageRecord{}, false
	}
	u := l.Message.Usage
	t := board.CostTokens{Input: u.Input, CacheRead: u.CacheRead, Output: u.Output}
	if u.Split != nil {
		t.CacheWrite5m, t.CacheWrite1h = u.Split.M5, u.Split.H1
		// Any total beyond the split is not attributed to 1h: it bills at 5m.
		if rest := u.CacheCreation - t.CacheWrite5m - t.CacheWrite1h; rest > 0 {
			t.CacheWrite5m += rest
		}
	} else {
		t.CacheWrite5m = u.CacheCreation
	}
	return usageRecord{ID: l.Message.ID, Model: l.Message.Model, Tokens: t}, true
}
