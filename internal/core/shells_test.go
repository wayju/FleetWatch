package core

import (
	"fmt"
	"time"
)

// Live background shells: a Bash/Monitor launch stays pending in the
// transcript until a <task-notification> names it, so one that ends without a
// notification would hold WAIT(C) forever. The scanner's per-pid live shell
// count caps it.

func shellDoc(scannedAt time.Time, shells string) string {
	return fmt.Sprintf(`{"scanned_at":%d,"servers":[],"shells":%s}`, scannedAt.Unix(), shells)
}
