package sysact

// DryRun returns a System that does nothing but tell what it was asked
// through say: for trying Gocast's actions, their confirmations and
// notices (GOCAST_ACTIONS_DRY=1), the Mac left as is. The volume is half
// and stays so; the Trash is never empty.
func DryRun(say func(format string, args ...any)) System { return dry{say} }

type dry struct{ say func(string, ...any) }

func (d dry) did(what string) error { d.say("sysact (dry run): %s", what); return nil }

func (d dry) LockScreen() error       { return d.did("lock screen") }
func (d dry) Sleep() error            { return d.did("sleep") }
func (d dry) SleepDisplays() error    { return d.did("sleep displays") }
func (d dry) StartScreenSaver() error { return d.did("screen saver") }
func (d dry) Restart() error          { return d.did("restart") }
func (d dry) ShutDown() error         { return d.did("shut down") }
func (d dry) LogOut() error           { return d.did("log out") }
func (d dry) Volume() (float64, bool, error) {
	return 0.5, false, nil
}
func (d dry) SetVolume(level float64) error { return d.did("set volume") }
func (d dry) SetMuted(muted bool) error     { return d.did("set muted") }
func (d dry) ToggleAppearance() (bool, error) {
	return false, d.did("toggle appearance")
}
func (d dry) ShowDesktop() error        { return d.did("show desktop") }
func (d dry) HideOthers(int) error      { return d.did("hide others") }
func (d dry) UnhideAll() error          { return d.did("unhide all") }
func (d dry) QuitAll() error            { return d.did("quit all") }
func (d dry) OpenTrash() error          { return d.did("open trash") }
func (d dry) TrashEmpty() (bool, error) { return false, nil }
func (d dry) EmptyTrash() error         { return d.did("empty trash") }
func (d dry) EjectAll() (int, []string, error) {
	return 0, nil, d.did("eject all")
}
func (d dry) ToggleHiddenFiles() (bool, error) {
	return false, d.did("toggle hidden files")
}
