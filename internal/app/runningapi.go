package app

import "gocast/internal/running"

// The other apps, as the system tells of them and acts on them, through
// these: the tests stand in for them (helpers_test.go), never asking
// the system, which a CI runner with no login session crashes on.
var (
	sysIsRunning = running.IsRunning
	sysAppPath   = running.AppPath
	sysFrontmost = running.Frontmost
	sysHide      = running.Hide
	sysOpen      = running.Open
	sysQuit      = running.Quit
)
