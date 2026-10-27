package board

// API-equivalent cost on the board: what a session's token usage would cost at
// API list prices. The sessions run on a plan, so this is a comparison between
// sessions and models, not a bill.

import "sort"

type CostTokens struct {
	Input        int64 `json:"input"`
	CacheWrite5m int64 `json:"cache_write_5m"`
	CacheWrite1h int64 `json:"cache_write_1h"`
	CacheRead    int64 `json:"cache_read"`
	Output       int64 `json:"output"`
}

// Add folds o into t.
func (t *CostTokens) Add(o CostTokens) {
	t.Input += o.Input
	t.CacheWrite5m += o.CacheWrite5m
	t.CacheWrite1h += o.CacheWrite1h
	t.CacheRead += o.CacheRead
	t.Output += o.Output
}

// ModelCost is one model's share of a session (or of the whole board).
// Unpriced marks a model missing from the price table: its tokens are counted
// and its USD is 0.
type ModelCost struct {
	Model    string     `json:"model"`
	Class    string     `json:"model_class,omitempty"` // display class from the models config
	USD      float64    `json:"usd"`
	Messages int64      `json:"messages"`
	Tokens   CostTokens `json:"tokens"`
	Unpriced bool       `json:"unpriced,omitempty"`
}

// Add folds o into m.
func (m *ModelCost) Add(o ModelCost) {
	m.USD += o.USD
	m.Messages += o.Messages
	m.Tokens.Add(o.Tokens)
	m.Unpriced = m.Unpriced || o.Unpriced
}

// SessionCost is SessionView.cost. Partial is true until every transcript of
// the session has had its first full read.
type SessionCost struct {
	USD        float64     `json:"usd"`
	Partial    bool        `json:"partial"`
	PricedAsOf string      `json:"priced_as_of"`
	Tokens     CostTokens  `json:"tokens"`
	ByModel    []ModelCost `json:"by_model"`
}

// BoardCost is Snapshot.cost, over the sessions on the board.
type BoardCost struct {
	USD        float64     `json:"usd"`
	ByModel    []ModelCost `json:"by_model"`
	PricedAsOf string      `json:"priced_as_of"`
}

// sortByUSD orders rows by cost descending, ties by model name so the board
// body stays stable between polls.
func sortByUSD(rows []ModelCost) {
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].USD != rows[j].USD {
			return rows[i].USD > rows[j].USD
		}
		return rows[i].Model < rows[j].Model
	})
}

// MergeModels sums rows by model into a new sorted slice.
func MergeModels(sets ...[]ModelCost) []ModelCost {
	by := map[string]*ModelCost{}
	for _, set := range sets {
		for _, r := range set {
			m := by[r.Model]
			if m == nil {
				m = &ModelCost{Model: r.Model, Class: r.Class}
				by[r.Model] = m
			}
			m.Add(r)
		}
	}
	out := make([]ModelCost, 0, len(by))
	for _, m := range by {
		out = append(out, *m)
	}
	sortByUSD(out)
	return out
}

// EmptyCost is a session cost with nothing counted yet.
func EmptyCost(asOf string) SessionCost {
	return SessionCost{PricedAsOf: asOf, ByModel: []ModelCost{}}
}

// TotalCost totals the sessions' costs by model.
func TotalCost(sessions []SessionView, asOf string) BoardCost {
	sets := make([][]ModelCost, 0, len(sessions))
	for _, s := range sessions {
		sets = append(sets, s.Cost.ByModel)
	}
	bc := BoardCost{PricedAsOf: asOf, ByModel: MergeModels(sets...)}
	for _, m := range bc.ByModel {
		bc.USD += m.USD
	}
	return bc
}
