//go:build !windows

package keyboard

import (
	"github.com/go-vgo/robotgo"
)

// platformType types text using robotgo
func platformType(text string) {
	robotgo.Type(text)
}

// platformKeyTap simulates a key tap using robotgo
func platformKeyTap(key string, args ...interface{}) {
	robotgo.KeyTap(key, args...)
}
