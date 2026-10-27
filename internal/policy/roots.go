// Package policy holds the repository-wide guard tests: no model literals in
// shipped code, and no ptrace-mediated /proc reads in runtime adapters. It has
// no production code; the constants let the tests find the tree from their own
// directory.
package policy

const (
	repoRoot = "../.."
	hostDir  = "../host"
)
