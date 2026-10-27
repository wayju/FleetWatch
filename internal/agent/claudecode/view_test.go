package claudecode

import (
	"testing"

	"github.com/wayju/fleetwatch/internal/models"
)

func TestContextOfUsesTheModelsTable(t *testing.T) {
	tbl, _ := models.New([]models.Entry{{Match: "known-*", ContextWindow: 1000, ExtendedWindow: 5000}})
	tr := &Transcript{Model: "known-1", ContextTokens: 250}
	if c := contextOf(tbl, tr); c.Limit != 1000 || c.Pct != 25 || c.Source != "transcript" {
		t.Errorf("known: %+v", c)
	}
	tr.ContextTokens = 2500
	if c := contextOf(tbl, tr); c.Limit != 5000 || c.Pct != 50 {
		t.Errorf("extended: %+v", c)
	}
	tr.Model = "mystery"
	if c := contextOf(tbl, tr); c.Limit != 0 || c.Pct != 0 || c.Source != "unknown" || c.Tokens != 2500 {
		t.Errorf("unknown model should report no window: %+v", c)
	}
	if c := contextOf(nil, tr); c.Source != "unknown" {
		t.Errorf("nil table: %+v", c)
	}
}
