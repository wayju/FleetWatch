// Package proc discovers agent processes from /proc, reading only stat,
// cmdline and cgroup. environ, the cwd link, fd and exe are ptrace-mediated
// under docker-default AppArmor and flood the audit queue (see ADR-007), so nothing
// here may touch them; File enforces the allowlist and forbidden_test.go in
// the runtime tree enforces it for every adapter.
package proc

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/wayju/fleetwatch/internal/host"
)

// clkTck is USER_HZ, which is 100 on every Linux this runs on.
const clkTck = 100

var allowed = map[string]bool{"stat": true, "cmdline": true, "cgroup": true}

// File reads one of the few /proc/<pid> files the fleet may read.
func File(root string, pid int, name string) ([]byte, error) {
	if !allowed[name] {
		return nil, fmt.Errorf("proc: reading %q is not allowed", name)
	}
	return os.ReadFile(filepath.Join(root, strconv.Itoa(pid), name))
}

func bootTime(procRoot string) time.Time {
	f, err := os.Open(filepath.Join(procRoot, "stat"))
	if err != nil {
		return time.Time{}
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		if v, ok := strings.CutPrefix(sc.Text(), "btime "); ok {
			n, _ := strconv.ParseInt(strings.TrimSpace(v), 10, 64)
			return time.Unix(n, 0)
		}
	}
	return time.Time{}
}

// readStat returns comm, ppid and start time (ticks since boot). comm may hold
// spaces and parens, so fields are counted from the last ')'.
func readStat(procRoot string, pid int) (comm string, ppid int, startTicks int64, ok bool) {
	b, err := File(procRoot, pid, "stat")
	if err != nil {
		return "", 0, 0, false
	}
	s := string(b)
	open, close := strings.IndexByte(s, '('), strings.LastIndexByte(s, ')')
	if open < 0 || close < open {
		return "", 0, 0, false
	}
	comm = s[open+1 : close]
	f := strings.Fields(s[close+1:])
	// f[0] is state (field 3); ppid is field 4, starttime field 22.
	if len(f) < 20 {
		return "", 0, 0, false
	}
	ppid, _ = strconv.Atoi(f[1])
	startTicks, _ = strconv.ParseInt(f[19], 10, 64)
	return comm, ppid, startTicks, true
}

// ParentOf is a process's parent pid, from its stat alone. Runtimes that map a
// terminal's pid to the agent below it (tmux) walk up with this.
func ParentOf(root string, pid int) (int, bool) {
	_, ppid, _, ok := readStat(root, pid)
	return ppid, ok
}

// Scan lists the processes the claimer accepts. A process whose parent is also
// accepted (a nested launcher shim) is folded into the outer one. The result is
// ordered by pid.
func Scan(root string, claim host.Claimer) []host.Session {
	ents, err := os.ReadDir(root)
	if err != nil {
		return nil
	}
	boot := bootTime(root)
	found := map[int]host.Session{}
	for _, e := range ents {
		pid, err := strconv.Atoi(e.Name())
		if err != nil {
			continue
		}
		comm, ppid, start, ok := readStat(root, pid)
		if !ok {
			continue
		}
		cmd := ""
		cmdline := func() string {
			if b, err := File(root, pid, "cmdline"); err == nil {
				cmd = strings.ReplaceAll(strings.TrimRight(string(b), "\x00"), "\x00", " ")
			}
			return cmd
		}
		agent, ok := claim(comm, cmdline)
		if !ok {
			continue
		}
		found[pid] = host.Session{Runtime: "proc", Agent: agent, PID: pid, PPID: ppid, Comm: comm, Cmdline: cmd,
			Start: boot.Add(time.Duration(start) * time.Second / clkTck)}
	}
	var out []host.Session
	for _, s := range found {
		if _, nested := found[s.PPID]; !nested {
			out = append(out, s)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].PID < out[j].PID })
	return out
}

// Runtime is the process-only runtime: it finds agent processes and knows
// nothing else about them.
type Runtime struct{ Root string }

// Name is the runtime's config type.
func (Runtime) Name() string { return "proc" }

// Sessions implements host.Runtime.
func (r Runtime) Sessions(_ time.Time, claim host.Claimer) []host.Session {
	return Scan(r.Root, claim)
}
