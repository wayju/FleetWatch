package herdr

import (
	"errors"
	"io/fs"
	"syscall"
)

// isNotRunning is true for the errors a dial gives when nothing listens on the
// socket: the file is absent, or it is stale and refuses connections.
func isNotRunning(err error) bool {
	return errors.Is(err, fs.ErrNotExist) || errors.Is(err, syscall.ECONNREFUSED) || errors.Is(err, syscall.ENOENT)
}
