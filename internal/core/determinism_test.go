package core

import (
	"bytes"
	"strconv"
)

// firstDiff names the first differing line of two marshalled snapshots.
func firstDiff(a, b []byte) string {
	la, lb := bytes.Split(a, []byte("\n")), bytes.Split(b, []byte("\n"))
	for i := 0; i < len(la) && i < len(lb); i++ {
		if !bytes.Equal(la[i], lb[i]) {
			return "line " + strconv.Itoa(i+1) + ":\n- " + string(la[i]) + "\n+ " + string(lb[i])
		}
	}
	return "lengths differ"
}
