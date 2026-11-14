//go:build !darwin

package sysact

// NewSystem returns the system's actions: none but on macOS.
func NewSystem(self ...string) System { return noSystem{} }

// NewWindows returns the system's windows: none but on macOS.
func NewWindows(screens func() []Screen) Windows { return noWindows{screens} }

type noSystem struct{}

func (noSystem) LockScreen() error                { return ErrUnsupported }
func (noSystem) Sleep() error                     { return ErrUnsupported }
func (noSystem) SleepDisplays() error             { return ErrUnsupported }
func (noSystem) StartScreenSaver() error          { return ErrUnsupported }
func (noSystem) Restart() error                   { return ErrUnsupported }
func (noSystem) ShutDown() error                  { return ErrUnsupported }
func (noSystem) LogOut() error                    { return ErrUnsupported }
func (noSystem) Volume() (float64, bool, error)   { return 0, false, ErrUnsupported }
func (noSystem) SetVolume(float64) error          { return ErrUnsupported }
func (noSystem) SetMuted(bool) error              { return ErrUnsupported }
func (noSystem) ToggleAppearance() (bool, error)  { return false, ErrUnsupported }
func (noSystem) ShowDesktop() error               { return ErrUnsupported }
func (noSystem) HideOthers(int) error             { return ErrUnsupported }
func (noSystem) UnhideAll() error                 { return ErrUnsupported }
func (noSystem) QuitAll() error                   { return ErrUnsupported }
func (noSystem) OpenTrash() error                 { return ErrUnsupported }
func (noSystem) TrashEmpty() (bool, error)        { return false, ErrUnsupported }
func (noSystem) EmptyTrash() error                { return ErrUnsupported }
func (noSystem) EjectAll() (int, []string, error) { return 0, nil, ErrUnsupported }
func (noSystem) ToggleHiddenFiles() (bool, error) { return false, ErrUnsupported }

type noWindows struct{ screens func() []Screen }

func (noWindows) Trusted() bool               { return false }
func (noWindows) Focused(int) (Window, error) { return nil, ErrUnsupported }
func (w noWindows) Screens() []Screen         { return w.screens() }
