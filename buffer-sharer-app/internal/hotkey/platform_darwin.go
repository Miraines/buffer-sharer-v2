//go:build darwin

package hotkey

import "golang.design/x/hotkey"

// modAlt is Option on macOS
const modAlt hotkey.Modifier = hotkey.ModOption

// modSuper is Cmd on macOS
const modSuper hotkey.Modifier = hotkey.ModCmd
