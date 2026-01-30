//go:build windows

package hotkey

import "golang.design/x/hotkey"

// modAlt is Alt key on Windows (Mod1 = 1 << 3)
const modAlt hotkey.Modifier = hotkey.Modifier(1 << 3)

// modSuper maps Cmd/Super to Ctrl on Windows for cross-platform compatibility
const modSuper hotkey.Modifier = hotkey.ModCtrl
