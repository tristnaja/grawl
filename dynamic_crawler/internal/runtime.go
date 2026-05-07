package internal

import "sync/atomic"

var quietMode atomic.Bool

func SetQuietMode(quiet bool) {
	quietMode.Store(quiet)
}

func IsQuietMode() bool {
	return quietMode.Load()
}
