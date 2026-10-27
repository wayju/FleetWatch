// env.go: the process environment the resolver reads, injectable for tests.
package config

import (
	"os"
	"os/exec"
)

// Env is everything the resolver reads from the process, so tests can supply
// their own.
type Env struct {
	Get         func(string) string
	UserHomeDir func() (string, error)
	Hostname    func() (string, error)
	ReadFile    func(string) ([]byte, error)
	UID         int
	LookPath    func(string) (string, error) // finds an executable on PATH; nil finds none
}

// OSEnv is the real process environment.
func OSEnv() Env {
	return Env{Get: os.Getenv, UserHomeDir: os.UserHomeDir, Hostname: os.Hostname, ReadFile: os.ReadFile, UID: os.Getuid(), LookPath: exec.LookPath}
}
