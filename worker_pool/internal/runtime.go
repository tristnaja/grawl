package internal

import (
	"sync/atomic"
	"time"
)

var quietMode atomic.Bool
var defaultRateDelayNS atomic.Int64

func init() {
	defaultRateDelayNS.Store((2 * time.Second).Nanoseconds())
}

func SetQuietMode(quiet bool) {
	quietMode.Store(quiet)
}

func IsQuietMode() bool {
	return quietMode.Load()
}

func SetDefaultRateDelay(delay time.Duration) {
	if delay <= 0 {
		delay = 2 * time.Second
	}

	defaultRateDelayNS.Store(delay.Nanoseconds())
}

func DefaultRateDelay() time.Duration {
	return time.Duration(defaultRateDelayNS.Load())
}
