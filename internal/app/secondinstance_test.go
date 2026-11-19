package app

import "testing"

// A Gocast opened before the running one is ready, as while it relaunches
// to update, has it show its search panel once it is, rather than crash
// it: there is no panel yet.
func TestSecondInstanceBeforeReady(t *testing.T) {
	g := &gocast{}
	g.secondInstance()
	if !g.showWhenReady {
		t.Error("a Gocast opened before this one was ready is forgotten")
	}
}
