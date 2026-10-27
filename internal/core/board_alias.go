package core

// Package main uses the wire types under their short names; the definitions
// live in internal/board.

import "github.com/wayju/fleetwatch/internal/board"

type (
	Snapshot       = board.Snapshot
	Counts         = board.Counts
	SessionView    = board.SessionView
	ContextView    = board.ContextView
	Recap          = board.Recap
	ToolView       = board.ToolView
	AgentView      = board.AgentView
	AccountView    = board.AccountView
	BranchView     = board.BranchView
	LinkView       = board.LinkView
	LogView        = board.LogView
	CostTokens     = board.CostTokens
	ModelCost      = board.ModelCost
	SessionCost    = board.SessionCost
	BoardCost      = board.BoardCost
	AgentTotals    = board.AgentTotals
	AgentEventView = board.AgentEventView
	WorktreeView   = board.WorktreeView
	WorktreeWriter = board.WorktreeWriter
	IssueRef       = board.IssueRef
	RepoInfo       = board.RepoInfo
	Account        = board.Account
	ServerView     = board.ServerView
	NextWake       = board.NextWake
)
