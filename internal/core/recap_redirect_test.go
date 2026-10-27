package core

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestRecapRefusesRedirectAndKeepsKeyOnHost(t *testing.T) {
	var followed, leaked atomic.Bool
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		followed.Store(true)
		if r.Header.Get("Authorization") != "" {
			leaked.Store(true)
		}
	}))
	defer target.Close()
	gw := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, target.URL+"/v1/chat/completions", http.StatusTemporaryRedirect)
	}))
	defer gw.Close()

	r := NewRecapper(gw.URL, "secret-key", "m", time.Minute)
	_, err := r.call("digest")
	if err == nil || !strings.Contains(err.Error(), "307") {
		t.Fatalf("want a 307 error, got %v", err)
	}
	if followed.Load() || leaked.Load() {
		t.Fatalf("redirect followed=%v, key leaked=%v", followed.Load(), leaked.Load())
	}
}

func TestRecapPromptNamesNoParticularAgent(t *testing.T) {
	if !strings.Contains(recapSystem, "coding-agent session") || strings.Contains(recapSystem, "Claude") {
		t.Fatalf("recap prompt is not agent-neutral: %q", recapSystem)
	}
}
