// Gocast is a launcher for macOS in the manner of Raycast, in MyGo's native
// UI: Option+Shift+Space shows a search panel over every app to find and
// open installed apps (by their English and Chinese names, and by pinyin)
// and terminal presets; Control+` drops a floating terminal from the top
// of the screen, whose tabs run the presets. It lives in the menu bar,
// without a Dock icon, and runs once. The app is in internal/app.
//
//	go run .
package main

import "gocast/internal/app"

func main() {
	app.Run()
}
