package claudecode

import (
	"fmt"
	"github.com/wayju/fleetwatch/internal/board"
	"github.com/wayju/fleetwatch/internal/models"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// --- helpers ---

type msgSpec struct {
	id, model       string
	in, cc, cr, out int64
	m5, h1          int64
	split           bool
}

func (m msgSpec) line() string {
	usage := fmt.Sprintf(`"input_tokens":%d,"cache_creation_input_tokens":%d,"cache_read_input_tokens":%d,"output_tokens":%d`, m.in, m.cc, m.cr, m.out)
	if m.split {
		usage += fmt.Sprintf(`,"cache_creation":{"ephemeral_5m_input_tokens":%d,"ephemeral_1h_input_tokens":%d}`, m.m5, m.h1)
	}
	// iterations repeats the same counts; they must not be added twice.
	usage += fmt.Sprintf(`,"iterations":[{"input_tokens":%d,"output_tokens":%d}]`, m.in, m.out)
	return fmt.Sprintf(`{"type":"assistant","message":{"id":%q,"model":%q,"usage":{%s}}}`, m.id, m.model, usage) + "\n"
}

func writeLines(t *testing.T, path string, flag int, lines ...string) {
	t.Helper()
	f, err := os.OpenFile(path, flag|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if _, err := f.WriteString(strings.Join(lines, "")); err != nil {
		t.Fatal(err)
	}
}

// examplePrices is the shipped sample table; nothing is compiled in (see ADR-005).
const examplePrices = "../../../examples/prices.json"

func testAcc() *CostAccumulator { return NewCostAccumulator(loadPrices(examplePrices)) }

// settle polls the accumulator, stepping its clock past the refresh interval,
// until the first pass is done and ok accepts the result.
func settle(t *testing.T, a *CostAccumulator, path string, ok func(board.SessionCost) bool) board.SessionCost {
	t.Helper()
	now := time.Unix(1_800_000_000, 0)
	var c board.SessionCost
	for i := 0; i < 400; i++ {
		now = now.Add(2 * costRefresh)
		c = a.Session(path, now)
		if !c.Partial && ok(c) {
			return c
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("cost never settled: %+v", c)
	return c
}

func anyCost(board.SessionCost) bool { return true }

func near(a, b float64) bool { return math.Abs(a-b) < 1e-9 }

func byModel(c board.SessionCost, model string) board.ModelCost {
	for _, m := range c.ByModel {
		if m.Model == model {
			return m
		}
	}
	return board.ModelCost{}
}

// --- tests ---

func TestCostDedupesByMessageID(t *testing.T) {
	p := filepath.Join(t.TempDir(), "s.jsonl")
	a := msgSpec{id: "msg_a", model: "claude-opus-5-5", in: 10, cc: 100, cr: 1000, out: 50}
	b := msgSpec{id: "msg_b", model: "claude-opus-5-5", in: 1, cc: 0, cr: 2000, out: 5}
	// One line per content block: three for msg_a, then msg_b.
	writeLines(t, p, os.O_TRUNC, a.line(), a.line(), a.line(), b.line())
	c := settle(t, testAcc(), p, anyCost)
	m := byModel(c, "claude-opus-5-5")
	if m.Messages != 2 {
		t.Fatalf("messages = %d, want 2", m.Messages)
	}
	want := board.CostTokens{Input: 11, CacheWrite5m: 100, CacheRead: 3000, Output: 55}
	if m.Tokens != want {
		t.Fatalf("tokens = %+v, want %+v", m.Tokens, want)
	}
}

func TestCostRepeatedLinesKeepFinalOutputTokens(t *testing.T) {
	p := filepath.Join(t.TempDir(), "s.jsonl")
	mk := func(out int64) string {
		return msgSpec{id: "msg_a", model: "claude-opus-5-5", in: 2, cr: 500, out: out}.line()
	}
	// The first line is written mid-stream with a partial output count.
	writeLines(t, p, os.O_TRUNC, mk(16), mk(16), mk(307),
		msgSpec{id: "msg_b", model: "claude-opus-5-5", out: 40}.line())
	c := settle(t, testAcc(), p, anyCost)
	m := byModel(c, "claude-opus-5-5")
	if m.Messages != 2 || m.Tokens.Output != 347 || m.Tokens.CacheRead != 500 {
		t.Fatalf("row = %+v, want 2 messages, output 347, cache_read 500", m)
	}
	if want := (4.0*2 + 0.20*500 + 20.0*347) / 1e6; !near(c.USD, want) {
		t.Fatalf("usd = %v, want %v", c.USD, want)
	}
}

func TestCostRepeatedLineReprices(t *testing.T) {
	p := filepath.Join(t.TempDir(), "s.jsonl")
	mk := func(in int64) string {
		return msgSpec{id: "msg_a", model: "claude-haiku-5-5", in: in, out: 10}.line()
	}
	writeLines(t, p, os.O_TRUNC, mk(60000), mk(100001)) // the second crosses the 100K tier
	c := settle(t, testAcc(), p, anyCost)
	if want := (100001*0.50 + 10*2.50) / 1e6; !near(c.USD, want) {
		t.Fatalf("usd = %v, want %v", c.USD, want)
	}
}

func TestCostCacheWriteSplit(t *testing.T) {
	p := filepath.Join(t.TempDir(), "s.jsonl")
	m := msgSpec{id: "msg_1", model: "claude-sonnet-5-5", cc: 1000, split: true, m5: 400, h1: 600}
	writeLines(t, p, os.O_TRUNC, m.line())
	c := settle(t, testAcc(), p, anyCost)
	if c.Tokens.CacheWrite5m != 400 || c.Tokens.CacheWrite1h != 600 {
		t.Fatalf("split = %d/%d, want 400/600", c.Tokens.CacheWrite5m, c.Tokens.CacheWrite1h)
	}
	want := (400*2.50 + 600*4.00) / 1e6
	if !near(c.USD, want) {
		t.Fatalf("usd = %v, want %v", c.USD, want)
	}
}

func TestCostMissingSplitIsFiveMinute(t *testing.T) {
	p := filepath.Join(t.TempDir(), "s.jsonl")
	m := msgSpec{id: "msg_1", model: "claude-sonnet-5-5", cc: 1000}
	writeLines(t, p, os.O_TRUNC, m.line())
	c := settle(t, testAcc(), p, anyCost)
	if c.Tokens.CacheWrite5m != 1000 || c.Tokens.CacheWrite1h != 0 {
		t.Fatalf("tokens = %+v, want all 1000 at 5m", c.Tokens)
	}
}

func TestCostHaikuLongContextTierPerMessage(t *testing.T) {
	p := filepath.Join(t.TempDir(), "s.jsonl")
	short := msgSpec{id: "msg_s", model: "claude-haiku-5-5", in: 1000, out: 1000}
	long := msgSpec{id: "msg_l", model: "claude-haiku-5-5", in: 50000, cr: 50001, out: 1000} // 100001 > 100K
	edge := msgSpec{id: "msg_e", model: "claude-haiku-5-5", in: 50000, cr: 50000, out: 1000} // exactly 100K: base tier
	writeLines(t, p, os.O_TRUNC, short.line(), long.line(), edge.line())
	c := settle(t, testAcc(), p, anyCost)
	base := func(in, cr, out float64) float64 { return (in*0.10 + cr*0.01 + out*0.50) / 1e6 }
	tier := (50000*0.50 + 50001*0.05 + 1000*2.50) / 1e6
	want := base(1000, 0, 1000) + tier + base(50000, 50000, 1000)
	if !near(c.USD, want) {
		t.Fatalf("usd = %v, want %v", c.USD, want)
	}
}

func TestCostUnknownModelCountsTokensNoUSD(t *testing.T) {
	p := filepath.Join(t.TempDir(), "s.jsonl")
	m := msgSpec{id: "msg_1", model: "<synthetic>", in: 7, cr: 9, out: 3}
	writeLines(t, p, os.O_TRUNC, m.line())
	c := settle(t, testAcc(), p, anyCost)
	row := byModel(c, "<synthetic>")
	if !row.Unpriced || row.USD != 0 || row.Tokens.Input != 7 || row.Tokens.CacheRead != 9 || row.Messages != 1 {
		t.Fatalf("row = %+v", row)
	}
	if c.USD != 0 {
		t.Fatalf("usd = %v, want 0", c.USD)
	}
}

func TestCostIncrementalAppendReadsOnlyNewBytes(t *testing.T) {
	p := filepath.Join(t.TempDir(), "s.jsonl")
	a := msgSpec{id: "msg_a", model: "claude-opus-5-5", out: 10}
	b := msgSpec{id: "msg_b", model: "claude-opus-5-5", out: 20}
	acc := testAcc()
	writeLines(t, p, os.O_TRUNC, a.line())
	settle(t, acc, p, anyCost)
	// b arrives in two writes: the unfinished line must wait for its newline.
	bl := b.line()
	writeLines(t, p, os.O_APPEND, a.line(), bl[:len(bl)/2])
	c := settle(t, acc, p, func(c board.SessionCost) bool { return c.Tokens.Output == 10 })
	if c.Tokens.Output != 10 {
		t.Fatalf("output = %d, want 10 (repeat of msg_a ignored, half line unread)", c.Tokens.Output)
	}
	writeLines(t, p, os.O_APPEND, bl[len(bl)/2:])
	c = settle(t, acc, p, func(c board.SessionCost) bool { return c.Tokens.Output == 30 })
	if c.Tokens.Output != 30 || byModel(c, "claude-opus-5-5").Messages != 2 {
		t.Fatalf("after append: %+v", c)
	}
}

func TestCostTruncationRescans(t *testing.T) {
	p := filepath.Join(t.TempDir(), "s.jsonl")
	acc := testAcc()
	writeLines(t, p, os.O_TRUNC,
		msgSpec{id: "msg_a", model: "claude-opus-5-5", out: 1000}.line(),
		msgSpec{id: "msg_b", model: "claude-opus-5-5", out: 2000}.line())
	settle(t, acc, p, func(c board.SessionCost) bool { return c.Tokens.Output == 3000 })
	// Rotated: a shorter file whose first message reuses an id already counted.
	writeLines(t, p, os.O_TRUNC, msgSpec{id: "msg_a", model: "claude-opus-5-5", out: 7}.line())
	c := settle(t, acc, p, func(c board.SessionCost) bool { return c.Tokens.Output == 7 })
	if m := byModel(c, "claude-opus-5-5"); m.Messages != 1 {
		t.Fatalf("messages = %d, want 1", m.Messages)
	}
}

func TestCostCountsSubagentTranscripts(t *testing.T) {
	dir := t.TempDir()
	main := filepath.Join(dir, "sess.jsonl")
	if err := os.MkdirAll(filepath.Join(dir, "sess", "subagents"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeLines(t, main, os.O_TRUNC, msgSpec{id: "msg_m", model: "claude-opus-5-5", out: 5}.line())
	writeLines(t, filepath.Join(dir, "sess", "subagents", "agent-x.jsonl"), os.O_TRUNC,
		msgSpec{id: "msg_s", model: "claude-sonnet-5-5", out: 11}.line())
	c := settle(t, testAcc(), main, anyCost)
	if c.Tokens.Output != 16 || len(c.ByModel) != 2 {
		t.Fatalf("cost = %+v", c)
	}
}

func TestCostByModelSortedByUSDDescending(t *testing.T) {
	p := filepath.Join(t.TempDir(), "s.jsonl")
	writeLines(t, p, os.O_TRUNC,
		msgSpec{id: "msg_1", model: "claude-haiku-5-5", out: 1000}.line(),
		msgSpec{id: "msg_2", model: "claude-fable-5-1", out: 1000}.line(),
		msgSpec{id: "msg_3", model: "claude-opus-5-5", out: 1000}.line())
	c := settle(t, testAcc(), p, anyCost)
	got := []string{c.ByModel[0].Model, c.ByModel[1].Model, c.ByModel[2].Model}
	if strings.Join(got, ",") != "claude-fable-5-1,claude-opus-5-5,claude-haiku-5-5" {
		t.Fatalf("order = %v", got)
	}
}

func TestCostMissingFileIsNotPartialForever(t *testing.T) {
	c := settle(t, testAcc(), filepath.Join(t.TempDir(), "gone.jsonl"), anyCost)
	if c.USD != 0 || len(c.ByModel) != 0 {
		t.Fatalf("cost = %+v", c)
	}
}

func TestIDRingForgetsOldestOnly(t *testing.T) {
	var r idRing
	for i := 0; i < costIDRing; i++ {
		if r.seen(fmt.Sprint("id", i)) {
			t.Fatalf("id%d reported seen on first sight", i)
		}
	}
	if !r.seen("id0") || !r.seen(fmt.Sprint("id", costIDRing-1)) {
		t.Fatal("ids inside the ring must be remembered")
	}
	r.seen("overflow") // evicts id0
	if r.seen("id0") {
		t.Fatal("id0 should have been evicted")
	}
}

func TestScanLinesCapsLongLinesAndLeavesPartialTail(t *testing.T) {
	long := strings.Repeat("x", 100)
	in := "one\n" + long + "\ntwo\npartial"
	var got []string
	n, skipped, err := scanLines(strings.NewReader(in), 50, func(b []byte) { got = append(got, string(b)) })
	if err != nil || skipped != 1 {
		t.Fatalf("err=%v skipped=%d", err, skipped)
	}
	if strings.Join(got, "|") != "one\n|two\n" {
		t.Fatalf("lines = %q", got)
	}
	if n != int64(len(in)-len("partial")) {
		t.Fatalf("consumed = %d", n)
	}
}

func TestParseUsageIgnoresNonUsageLines(t *testing.T) {
	for _, l := range []string{`{"type":"user","message":{"content":"usage"}}`, `not json "usage"`, `{"type":"x","toolUseResult":{"usage":{"output_tokens":9}}}`} {
		if _, ok := parseUsage([]byte(l)); ok {
			t.Errorf("line %q should carry no usage", l)
		}
	}
}

func TestPricesOverrideFileAndFallback(t *testing.T) {
	dir := t.TempDir()
	good := filepath.Join(dir, "p.json")
	os.WriteFile(good, []byte(`{"as_of":"2030-01-01","models":{"claude-opus-5-5":{"input":1,"output":1,"cache_write_5m":1,"cache_write_1h":1,"cache_read":1}}}`), 0o644)
	if pt := loadPrices(good); pt.AsOf != "2030-01-01" {
		t.Fatalf("override not used: %s", pt.AsOf)
	}
	bad := filepath.Join(dir, "bad.json")
	os.WriteFile(bad, []byte(`{nope`), 0o644)
	if pt := loadPrices(bad); len(pt.Models) != 0 {
		t.Fatalf("a bad price file should price nothing, got %d models", len(pt.Models))
	}
	if pt := loadPrices(filepath.Join(dir, "missing.json")); len(pt.Models) != 0 {
		t.Fatal("a missing price file should price nothing")
	}
}

// The binary carries no prices: with no file, a model's tokens are counted
// and its USD is 0 and flagged unpriced.
func TestNoPriceFilePricesNothing(t *testing.T) {
	pt := loadPrices("")
	if len(pt.Models) != 0 || pt.AsOf != "" {
		t.Fatalf("empty path should give an empty table, got %+v", pt)
	}
	if usd, ok := pt.price("claude-opus-5-5", board.CostTokens{Input: 1000, Output: 1000}); ok || usd != 0 {
		t.Fatalf("priced %v %v with no table", usd, ok)
	}
	p := filepath.Join(t.TempDir(), "s.jsonl")
	writeLines(t, p, os.O_TRUNC, msgSpec{id: "m1", model: "claude-opus-5-5", in: 1000, out: 500}.line())
	got := settle(t, NewCostAccumulator(pt), p, anyCost)
	if got.USD != 0 || got.Tokens.Input != 1000 || len(got.ByModel) != 1 || !got.ByModel[0].Unpriced {
		t.Fatalf("want tokens and unpriced, got %+v", got)
	}
}

// by_model rows carry the class from the models table, unclassed without one.
func TestCostByModelCarriesClass(t *testing.T) {
	tbl, err := models.New([]models.Entry{{Match: "claude-opus-*", ContextWindow: 200000, Class: "big"}})
	if err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(t.TempDir(), "s.jsonl")
	writeLines(t, p, os.O_TRUNC, msgSpec{id: "a", model: "claude-opus-5-5", out: 5}.line(), msgSpec{id: "b", model: "claude-haiku-5-5", out: 5}.line())
	a := testAcc()
	a.models = tbl
	c := settle(t, a, p, anyCost)
	if got := byModel(c, "claude-opus-5-5").Class; got != "big" {
		t.Errorf("opus class %q, want big", got)
	}
	if got := byModel(c, "claude-haiku-5-5").Class; got != "" {
		t.Errorf("unmatched class %q, want empty", got)
	}
}

func TestPriceLookupVariants(t *testing.T) {
	pt := loadPrices(examplePrices)
	for _, m := range []string{"claude-opus-5-5", "claude-opus-5-5-20261001", "claude-opus-5-5[1m]"} {
		if _, ok := pt.lookup(m); !ok {
			t.Errorf("%s should price", m)
		}
	}
	for _, m := range []string{"claude-opus-5-50", "<synthetic>", ""} {
		if _, ok := pt.lookup(m); ok {
			t.Errorf("%s should be unpriced", m)
		}
	}
}
