package main

// The automation plugin lets mygo-automation drive the app (tests, agents)
// through the socket MYGO_AUTOMATION names. It listens only when that is
// set, and release builds of mygo build (-tags mygo_noinspector) leave it
// out.
import _ "github.com/egoist/mygo/plugins/automation"
