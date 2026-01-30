//go:build linux

package hotkey

import "golang.design/x/hotkey"

// modAlt is Alt key on Linux (Mod1 = 1 << 3)
const modAlt hotkey.Modifier = hotkey.Modifier(1 << 3)

// modSuper maps Cmd/Super to Ctrl on Linux for cross-platform compatibility
const modSuper hotkey.Modifier = hotkey.ModCtrl
