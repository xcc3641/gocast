// Package icon holds Gocast's icons, which make-icon.py beside them draws:
// AppIcon.icon and icon.png are the app's (mygo.config.ts), and the PNGs below
// are embedded for the app to draw.
package icon

import _ "embed"

// App is the app icon, 1024 px (icon.png).
//
//go:embed icon.png
var App []byte

// Tray is the menu-bar icon: Gocast's mark, 18 DIP (36 px), black on
// transparent, a template image.
//
//go:embed tray.png
var Tray []byte

// Logo is Gocast's mark, 20 DIP (40 px), black on transparent, a
// template image.
//
//go:embed logo-template.png
var Logo []byte
