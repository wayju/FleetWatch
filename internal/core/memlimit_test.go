package core

import "testing"

func TestApplyMemoryLimit(t *testing.T) {
	var set int64 = -1
	rec := func(n int64) int64 { set = n; return 0 }

	for name, c := range map[string]struct {
		mib     int
		env     string
		applied bool
		want    int64
	}{
		"zero sets nothing":        {0, "", false, -1},
		"negative sets nothing":    {-3, "", false, -1},
		"configured value applies": {192, "", true, 192 << 20},
		"GOMEMLIMIT wins":          {192, "1GiB", false, -1},
	} {
		set = -1
		if got := applyMemoryLimit(c.mib, c.env, rec); got != c.applied || set != c.want {
			t.Errorf("%s: applied %v set %d, want %v %d", name, got, set, c.applied, c.want)
		}
	}
}
