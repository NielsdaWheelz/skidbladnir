// Package boottime measures elapsed time including system sleep.
package boottime

import (
	"time"

	"golang.org/x/sys/unix"
)

// Now returns a local receipt or update time. Zero means the clock was unavailable.
func Now() time.Duration {
	var stamp unix.Timespec
	if err := unix.ClockGettime(clockID, &stamp); err != nil {
		return 0
	}
	return time.Duration(stamp.Nano())
}
