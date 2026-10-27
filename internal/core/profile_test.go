package core

// Helpers shared by the end-to-end tests: the shipped prices file, the model
// table the goldens were recorded with, and the Claude Code agent built from a
// Config.

import (
	"os"
	"path/filepath"

	"github.com/wayju/fleetwatch/internal/agent/claudecode"
	"github.com/wayju/fleetwatch/internal/host"
	"github.com/wayju/fleetwatch/internal/models"
)

// testPricesFile is the shipped example table; the binary carries none (see ADR-005).
func testPricesFile() string {
	if p := os.Getenv("FLEET_PRICES_FILE"); p != "" {
		return p
	}
	return filepath.Join("..", "..", "examples", "prices.json")
}

// testModels sizes context windows the way the goldens were recorded: 200K,
// or 1M for an extended-context id or once usage passes 200K. It sets no
// class, so the golden boards carry no model_class.
func testModels() *models.Table {
	t, err := models.New([]models.Entry{
		{Match: "*\\[1m\\]", ContextWindow: 1_000_000},
		{Match: "*", ContextWindow: 200_000, ExtendedWindow: 1_000_000},
	})
	if err != nil {
		panic(err)
	}
	return t
}

// modelsFor is the config's own table, else the golden-compatible default.
func modelsFor(cfg Config) *models.Table {
	if cfg.Models != nil {
		return cfg.Models
	}
	return testModels()
}

// claudecodeAgent is the Claude Code agent this Config describes: transcripts
// under ~/.claude plus the config dirs the runtimes know about.
func claudecodeAgent(cfg Config, rts runtimeSet) *claudecode.Agent {
	return claudecode.New(claudecode.Config{
		Dirs:      []host.ConfigDir{{Path: filepath.Join(cfg.Home, ".claude")}},
		ExtraDirs: rts.configDirs,
		Window:    cfg.Window, RescanEvery: cfg.RescanEvery,
		PricesFile: testPricesFile(), Models: modelsFor(cfg),
	})
}
