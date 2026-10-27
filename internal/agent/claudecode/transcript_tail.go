package claudecode

// The file-following half of the transcript reader: tail tracks how far one
// JSONL file has been read and feeds each complete appended line to Apply.

import (
	"bufio"
	"bytes"
	"github.com/wayju/fleetwatch/internal/fsafe"
	"io"
	"syscall"
	"time"
)

// tail tracks how far one file has been read.
// tail follows one transcript file and folds what is appended into T.
type tail struct {
	path     string
	offset   int64
	ino      uint64
	partial  []byte // the unfinished last line, never over maxLine
	skipping bool   // inside a line already known to be over maxLine (or a cut first line)
	opened   bool   // the file has been polled once since it was (re)started
	T        *Transcript
	seen     time.Time // last poll that found the file
}

func newTail(path, id string) *tail {
	return &tail{path: path, T: NewTranscript(id)}
}

// poll reads whatever was appended since the last call and folds it in. At
// most maxReadPoll bytes are read per call, through a line reader that holds
// one line at a time: a line over maxLine is skipped, and the file's first
// open starts at its last maxInitial bytes.
func (tl *tail) poll(now time.Time) error {
	f, st, err := fsafe.OpenRegular(tl.path)
	if err != nil {
		return err
	}
	defer f.Close()
	tl.seen = now
	var ino uint64
	if s, ok := st.Sys().(*syscall.Stat_t); ok {
		ino = s.Ino
	}
	if st.Size() < tl.offset || (tl.ino != 0 && ino != tl.ino) {
		// Truncated or replaced: start over rather than fold a new file onto
		// the old one's state.
		tl.offset, tl.partial, tl.skipping, tl.opened, tl.T = 0, nil, false, false, NewTranscript(tl.T.ID)
	}
	tl.ino = ino
	if !tl.opened {
		tl.opened = true
		if st.Size() > maxInitial {
			tl.offset, tl.skipping = st.Size()-maxInitial, true // lands mid-line: drop to the next newline
		}
	}
	if st.Size() == tl.offset {
		return nil
	}
	if _, err := f.Seek(tl.offset, io.SeekStart); err != nil {
		return err
	}
	br := bufio.NewReaderSize(io.LimitReader(f, maxReadPoll), 64<<10)
	for {
		chunk, err := br.ReadSlice('\n')
		tl.offset += int64(len(chunk))
		if len(chunk) > 0 {
			tl.feed(chunk, len(chunk) > 0 && chunk[len(chunk)-1] == '\n')
		}
		if err == bufio.ErrBufferFull {
			continue
		}
		if err != nil {
			if err != io.EOF {
				return err
			}
			return nil
		}
	}
}

// feed takes one chunk of the line being read; complete says it ended in a
// newline, which is when the line is applied.
func (tl *tail) feed(chunk []byte, complete bool) {
	if tl.skipping {
		if complete {
			tl.skipping = false
		}
		return
	}
	if len(tl.partial)+len(chunk) > maxLine {
		tl.partial, tl.skipping = nil, !complete
		return
	}
	tl.partial = append(tl.partial, chunk...)
	if !complete {
		return
	}
	if line := bytes.TrimSpace(tl.partial); len(line) > 0 {
		tl.T.Apply(line)
	}
	tl.partial = nil
}
