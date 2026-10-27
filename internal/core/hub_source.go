package core

// Hub mode (see ADR-008): the board is the merge of this host (optionally) and the
// boards of the configured upstream instances. The serving side is unchanged;
// only the producer behind runSource differs.

import (
	"context"
	"github.com/wayju/fleetwatch/internal/board"
	"github.com/wayju/fleetwatch/internal/config"
	"github.com/wayju/fleetwatch/internal/upstream"
	"time"
)

type hubSource struct {
	host    string
	recent  time.Duration
	col     *Collector // nil unless hub.local
	sources []*upstream.Source
}

func newHubSource(res *config.Resolved, col *Collector) *hubSource {
	h := &hubSource{host: res.Host, recent: res.Recent, col: col}
	for _, u := range res.Hub.Upstreams {
		s := upstream.NewSource(u)
		s.MaxEventBytes = res.Hub.MaxEventBytes
		h.sources = append(h.sources, s)
	}
	return h
}

// start follows every upstream; onBoard fires after each new board so the poll
// loop merges at once instead of at the next tick.
func (h *hubSource) start(ctx context.Context, onBoard func()) {
	for _, s := range h.sources {
		go s.Run(ctx, onBoard)
	}
}

func (h *hubSource) parts(now time.Time) []upstream.Part {
	parts := make([]upstream.Part, 0, len(h.sources))
	for _, s := range h.sources {
		parts = append(parts, s.Part(now, h.recent))
	}
	return parts
}

func (h *hubSource) collect(now time.Time) Snapshot {
	var local *Snapshot
	if h.col != nil {
		s := h.col.Collect(now)
		local = &s
	}
	return upstream.Merge(h.host, local, h.parts(now), now)
}

func (h *hubSource) views() []board.UpstreamView {
	now := time.Now()
	out := []board.UpstreamView{}
	for _, p := range h.parts(now) {
		out = append(out, board.UpstreamView{Name: p.Name, State: p.State, LastSeen: board.Unix(p.LastSeen), Error: p.Err})
	}
	return out
}
