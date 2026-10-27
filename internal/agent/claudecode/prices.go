package claudecode

// API list prices used for the API-equivalent cost on the board. They are
// config, not a live pull and not compiled in (see ADR-005): cost.prices in the fleet
// config, or FLEET_PRICES_FILE, names the file (examples/prices.json is the
// shipped sample). Without one nothing is priced and cost reports tokens with
// unpriced set.

import (
	"encoding/json"
	"fmt"
	"github.com/wayju/fleetwatch/internal/board"
	"log"
	"os"
	"strings"
)

// Rates are USD per million tokens.
type Rates struct {
	Input        float64 `json:"input"`
	Output       float64 `json:"output"`
	CacheWrite5m float64 `json:"cache_write_5m"`
	CacheWrite1h float64 `json:"cache_write_1h"`
	CacheRead    float64 `json:"cache_read"`
}

// LongContext replaces a model's rates for a message whose prompt (input +
// cache read + cache write) exceeds Threshold tokens.
type LongContext struct {
	Threshold int64 `json:"threshold"`
	Rates
}

type ModelPrice struct {
	Rates
	LongContext *LongContext `json:"long_context,omitempty"`
}

type PriceTable struct {
	AsOf   string                `json:"as_of"`
	Source string                `json:"source,omitempty"`
	Models map[string]ModelPrice `json:"models"`
}

func parsePrices(b []byte) (*PriceTable, error) {
	var pt PriceTable
	if err := json.Unmarshal(b, &pt); err != nil {
		return nil, err
	}
	if len(pt.Models) == 0 {
		return nil, fmt.Errorf("no models")
	}
	for name, m := range pt.Models {
		if m.Input < 0 || m.Output < 0 || m.CacheWrite5m < 0 || m.CacheWrite1h < 0 || m.CacheRead < 0 {
			return nil, fmt.Errorf("model %s: negative rate", name)
		}
	}
	return &pt, nil
}

// loadPrices reads the price file at path. An empty path, or one that is
// missing or invalid, gives a table that prices nothing (a bad file is
// logged): wrong prices are worse than none, and a missing board is worse than
// either.
func loadPrices(path string) *PriceTable {
	none := &PriceTable{Models: map[string]ModelPrice{}}
	if path == "" {
		return none
	}
	b, err := os.ReadFile(path)
	if err == nil {
		var pt *PriceTable
		if pt, err = parsePrices(b); err == nil {
			return pt
		}
	}
	log.Printf("prices: ignoring %s: %v", path, err)
	return none
}

// lookup finds the price for a transcript model id: exact, else the longest
// table key that prefixes it at a boundary (a dated or [1m] variant).
func (pt *PriceTable) lookup(model string) (ModelPrice, bool) {
	if m, ok := pt.Models[model]; ok {
		return m, true
	}
	best := ""
	for k := range pt.Models {
		if strings.HasPrefix(model, k) && len(k) > len(best) && (model[len(k)] == '-' || model[len(k)] == '[') {
			best = k
		}
	}
	if best == "" {
		return ModelPrice{}, false
	}
	return pt.Models[best], true
}

// price returns the USD for one message's usage, and false for an unknown model.
func (pt *PriceTable) price(model string, t board.CostTokens) (float64, bool) {
	mp, ok := pt.lookup(model)
	if !ok {
		return 0, false
	}
	r := mp.Rates
	if lc := mp.LongContext; lc != nil && t.Input+t.CacheRead+t.CacheWrite5m+t.CacheWrite1h > lc.Threshold {
		r = lc.Rates
	}
	usd := float64(t.Input)*r.Input + float64(t.Output)*r.Output +
		float64(t.CacheWrite5m)*r.CacheWrite5m + float64(t.CacheWrite1h)*r.CacheWrite1h +
		float64(t.CacheRead)*r.CacheRead
	return usd / 1e6, true
}
