package config

import (
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/wayju/fleetwatch/internal/host"
	"github.com/wayju/fleetwatch/internal/host/herdr"
	"github.com/wayju/fleetwatch/internal/registry"
)

// defaultHerdrSocket is herdr's documented default socket (socket-api.mdx,
// "Socket paths"); herdr's own HERDR_SOCKET_PATH overrides it.
const defaultHerdrSocket = "~/.config/herdr/herdr.sock"

type herdrDoc struct {
	Type   string `json:"type"`
	Socket string `json:"socket"`
	Rank   *int   `json:"rank"`
}

func decodeHerdr(res registry.Resolver, raw json.RawMessage, i int) (RuntimeSpec, error) {
	var d herdrDoc
	if err := registry.Strict(raw, &d); err != nil {
		return RuntimeSpec{}, err
	}
	k := "runtimes[" + strconv.Itoa(i) + "]."
	s := RuntimeSpec{Type: "herdr", Rank: herdr.RankScreen}
	s.Socket = res.Pick(k+"socket", "HERDR_SOCKET_PATH", d.Socket, defaultHerdrSocket)
	res.Record(k+"rank", "default")
	if d.Rank != nil {
		// Screen-detected states must stay under the HUD projection and above the
		// transcript; a state herdr takes from the agent's hooks is always native.
		if *d.Rank <= host.RankTranscript || *d.Rank >= host.RankProjection {
			return RuntimeSpec{}, fmt.Errorf("rank must be above %d and below %d, got %d", host.RankTranscript, host.RankProjection, *d.Rank)
		}
		s.Rank = *d.Rank
		res.Record(k+"rank", "file")
	}
	return s, nil
}
