//go:build darwin

package sysact

import "github.com/ebitengine/purego/objc"

var selPressedButtons = objc.RegisterName("pressedMouseButtons")

// MouseHeld reports whether a mouse button is down now, wherever the
// pointer is (NSEvent's pressedMouseButtons): as the window server still
// moves a window the user drags, whose release reaches no app.
func MouseHeld() bool {
	if !appKitOnce() {
		return false
	}
	return objc.Send[uint](objc.ID(objc.GetClass("NSEvent")), selPressedButtons) != 0
}
