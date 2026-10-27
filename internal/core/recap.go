package core

// A short "what is this session doing" summary, written outside the session.
// Nothing is typed into a live pane: keystrokes sent to a working session
// queue up as its next prompt, and a waiting one would spend its turn on
// them. Instead the fleet sends a redacted digest of the transcript to the LLM
// gateway (an OpenAI-compatible endpoint) and keeps the answer. There is no
// default model: recap.url, recap.model and FLEET_RECAP_KEY must all be set,
// or the recap stays off (see ADR-005).
//
// One call at a time, only for live sessions that moved since their last
// summary, and no sooner than FLEET_RECAP_EVERY after it.

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/wayju/fleetwatch/internal/httpsafe"
	"github.com/wayju/fleetwatch/internal/textsafe"
	"net/http"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"
)

type recapEntry struct {
	text   string
	at     time.Time // when the summary was written
	asOf   time.Time // the session's LastActivity it describes
	failed time.Time
}

type Recapper struct {
	url, key, model string
	every           time.Duration
	http            *http.Client

	mu      sync.Mutex
	entries map[string]*recapEntry
	busy    bool
	calls   int
	errs    int
	lastErr string
}

func NewRecapper(url, key, model string, every time.Duration) *Recapper {
	if url == "" || key == "" || model == "" {
		return nil
	}
	return &Recapper{
		url: strings.TrimSuffix(url, "/"), key: key, model: model, every: every,
		http: &http.Client{Timeout: 90 * time.Second, CheckRedirect: httpsafe.RefuseRedirect}, entries: map[string]*recapEntry{},
	}
}

// Get returns the stored summary and when it was written.
func (r *Recapper) Get(id string) (string, int64) {
	if r == nil {
		return "", 0
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if e := r.entries[id]; e != nil && e.text != "" {
		return e.text, e.at.Unix()
	}
	return "", 0
}

type recapJob struct {
	id     string
	asOf   time.Time
	digest string
}

// Offer picks at most one session due for a summary and starts it.
func (r *Recapper) Offer(now time.Time, jobs []recapJob) {
	if r == nil || len(jobs) == 0 {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.busy {
		return
	}
	// The stalest summary goes first.
	sort.Slice(jobs, func(i, j int) bool { return r.writtenAt(jobs[i].id).Before(r.writtenAt(jobs[j].id)) })
	for _, j := range jobs {
		e := r.entries[j.id]
		if e != nil {
			if !j.asOf.After(e.asOf) || now.Sub(e.at) < r.every || now.Sub(e.failed) < r.every {
				continue
			}
		}
		r.busy = true
		go r.run(j)
		return
	}
}

func (r *Recapper) writtenAt(id string) time.Time {
	if e := r.entries[id]; e != nil {
		return e.at
	}
	return time.Time{}
}

var thinkRe = regexp.MustCompile(`(?s)<think>.*?</think>`)

const recapSystem = "You summarise a coding-agent session for a person watching many sessions at once. " +
	"In at most two short sentences (under 45 words), say what the session is working on and where it stands: " +
	"done, in progress, blocked, or waiting on the person and for what. Plain text, no preamble, no markdown."

func (r *Recapper) run(j recapJob) {
	text, err := r.call(j.digest)
	r.mu.Lock()
	defer r.mu.Unlock()
	r.busy = false
	r.calls++
	e := r.entries[j.id]
	if e == nil {
		e = &recapEntry{}
		r.entries[j.id] = e
	}
	if err != nil {
		r.errs++
		r.lastErr = err.Error()
		e.failed = time.Now()
		return
	}
	e.text, e.at, e.asOf, e.failed = text, time.Now(), j.asOf, time.Time{}
}

func (r *Recapper) call(digest string) (string, error) {
	body, _ := json.Marshal(map[string]any{
		"model":       r.model,
		"temperature": 0.2,
		"max_tokens":  160,
		"messages": []map[string]string{
			{"role": "system", "content": recapSystem},
			{"role": "user", "content": digest},
		},
	})
	req, err := http.NewRequest(http.MethodPost, r.url+"/v1/chat/completions", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+r.key)
	res, err := r.http.Do(req)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return "", fmt.Errorf("gateway returned %d", res.StatusCode)
	}
	var out struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.NewDecoder(res.Body).Decode(&out); err != nil {
		return "", err
	}
	if len(out.Choices) == 0 {
		return "", fmt.Errorf("gateway returned no choices")
	}
	text := strings.TrimSpace(thinkRe.ReplaceAllString(out.Choices[0].Message.Content, ""))
	if text == "" {
		return "", fmt.Errorf("empty summary")
	}
	return textsafe.Clip(textsafe.Redact(text), 400), nil
}

func (r *Recapper) Health() map[string]any {
	if r == nil {
		return map[string]any{"status": "disabled"}
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	return map[string]any{"status": "enabled", "model": r.model, "calls": r.calls, "errors": r.errs, "last_error": r.lastErr, "summaries": len(r.entries)}
}
