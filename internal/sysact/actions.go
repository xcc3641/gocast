// Package sysact is Gocast's actions (design.md 2.9, copy.md 15): the
// system's (lock the screen, sleep, the volume, the Trash, ...) and the
// window's (arrange the window in front into a half, a third, ...).
//
// What the actions ask of macOS goes through two interfaces, System and
// Windows, which the app's tests replace with fakes: no test may lock,
// sleep or restart the Mac, nor move the user's windows. The geometry of
// the window actions (Target, Arranger) is plain Go, tested with fake
// screens and windows.
package sysact

// Kind is what an action acts on: the system, or the window in front.
type Kind int

const (
	KindSystem Kind = iota + 1
	KindWindow
)

// Action is one of Gocast's actions.
type Action struct {
	// ID is stable: settings.json keeps hotkeys by it, usage.json scores
	// it as "action:" + ID.
	ID   string
	Kind Kind
	// Symbol is its SF Symbol; "" for a window action drawn as a
	// diagram of where it puts the window (design.md 2.9).
	Symbol string
	// Confirm: the system's alert asks first (copy.md 15.3).
	Confirm bool
}

// The IDs of the actions, in copy.md 15.1 and 15.2's order.
const (
	LockScreen        = "lock-screen"
	Sleep             = "sleep"
	SleepDisplays     = "sleep-displays"
	ScreenSaver       = "screen-saver"
	Restart           = "restart"
	ShutDown          = "shut-down"
	LogOut            = "log-out"
	ToggleMute        = "toggle-mute"
	VolumeUp          = "volume-up"
	VolumeDown        = "volume-down"
	Volume0           = "volume-0"
	Volume25          = "volume-25"
	Volume50          = "volume-50"
	Volume75          = "volume-75"
	Volume100         = "volume-100"
	ToggleAppearance  = "toggle-appearance"
	ShowDesktop       = "show-desktop"
	HideOthers        = "hide-others"
	UnhideAll         = "unhide-all"
	QuitAll           = "quit-all"
	OpenTrash         = "open-trash"
	EmptyTrash        = "empty-trash"
	EjectAll          = "eject-all"
	ToggleHiddenFiles = "toggle-hidden-files"

	LeftHalf        = "left-half"
	RightHalf       = "right-half"
	TopHalf         = "top-half"
	BottomHalf      = "bottom-half"
	TopLeft         = "top-left"
	TopRight        = "top-right"
	BottomLeft      = "bottom-left"
	BottomRight     = "bottom-right"
	LeftThird       = "left-third"
	CenterThird     = "center-third"
	RightThird      = "right-third"
	LeftTwoThirds   = "left-two-thirds"
	RightTwoThirds  = "right-two-thirds"
	Maximize        = "maximize"
	AlmostMaximize  = "almost-maximize"
	ReasonableSize  = "reasonable-size"
	Center          = "center"
	MakeLarger      = "larger"
	MakeSmaller     = "smaller"
	Restore         = "restore"
	NextDisplay     = "next-display"
	PreviousDisplay = "previous-display"
)

// Actions are all the actions, in copy.md's order: the system's 24, then
// the window's 22. Settings › Shortcuts lists them in this order.
var Actions = []Action{
	{LockScreen, KindSystem, "lock", false},
	{Sleep, KindSystem, "moon.zzz", false},
	{SleepDisplays, KindSystem, "display", false},
	{ScreenSaver, KindSystem, "sparkles.tv", false},
	{Restart, KindSystem, "arrow.clockwise", true},
	{ShutDown, KindSystem, "power", true},
	{LogOut, KindSystem, "rectangle.portrait.and.arrow.right", true},
	{ToggleMute, KindSystem, "speaker.slash", false},
	{VolumeUp, KindSystem, "speaker.plus", false},
	{VolumeDown, KindSystem, "speaker.minus", false},
	{Volume0, KindSystem, "speaker", false},
	{Volume25, KindSystem, "speaker.wave.1", false},
	{Volume50, KindSystem, "speaker.wave.2", false},
	{Volume75, KindSystem, "speaker.wave.3", false},
	{Volume100, KindSystem, "speaker.wave.3", false},
	{ToggleAppearance, KindSystem, "circle.lefthalf.filled", false},
	{ShowDesktop, KindSystem, "menubar.dock.rectangle", false},
	{HideOthers, KindSystem, "eye.slash", false},
	{UnhideAll, KindSystem, "eye", false},
	{QuitAll, KindSystem, "xmark.circle", true},
	{OpenTrash, KindSystem, "trash", false},
	{EmptyTrash, KindSystem, "xmark.bin", true},
	{EjectAll, KindSystem, "eject", false},
	{ToggleHiddenFiles, KindSystem, "eye.circle", false},

	{LeftHalf, KindWindow, "", false},
	{RightHalf, KindWindow, "", false},
	{TopHalf, KindWindow, "", false},
	{BottomHalf, KindWindow, "", false},
	{TopLeft, KindWindow, "", false},
	{TopRight, KindWindow, "", false},
	{BottomLeft, KindWindow, "", false},
	{BottomRight, KindWindow, "", false},
	{LeftThird, KindWindow, "", false},
	{CenterThird, KindWindow, "", false},
	{RightThird, KindWindow, "", false},
	{LeftTwoThirds, KindWindow, "", false},
	{RightTwoThirds, KindWindow, "", false},
	{Maximize, KindWindow, "", false},
	{AlmostMaximize, KindWindow, "", false},
	{ReasonableSize, KindWindow, "", false},
	{Center, KindWindow, "", false},
	{MakeLarger, KindWindow, "arrow.up.left.and.arrow.down.right", false},
	{MakeSmaller, KindWindow, "arrow.down.right.and.arrow.up.left", false},
	{Restore, KindWindow, "arrow.uturn.backward", false},
	{NextDisplay, KindWindow, "display.2", false},
	{PreviousDisplay, KindWindow, "display.2", false},
}

// Find returns the action with the ID.
func Find(id string) (Action, bool) {
	for _, a := range Actions {
		if a.ID == id {
			return a, true
		}
	}
	return Action{}, false
}

// VolumeLevel is the volume, 0 to 100, that the action with the ID sets,
// or -1 for none (Volume0 … Volume100).
func VolumeLevel(id string) int {
	switch id {
	case Volume0:
		return 0
	case Volume25:
		return 25
	case Volume50:
		return 50
	case Volume75:
		return 75
	case Volume100:
		return 100
	}
	return -1
}

// Diagram is where a window action puts the window, in the 12×8 inner area
// of its icon (design.md 2.9): x, y, width, height; ok is false for the
// actions drawn with a symbol instead.
func Diagram(id string) (x, y, w, h float32, ok bool) {
	switch id {
	case LeftHalf:
		return 0, 0, 6, 8, true
	case RightHalf:
		return 6, 0, 6, 8, true
	case TopHalf:
		return 0, 0, 12, 4, true
	case BottomHalf:
		return 0, 4, 12, 4, true
	case TopLeft:
		return 0, 0, 6, 4, true
	case TopRight:
		return 6, 0, 6, 4, true
	case BottomLeft:
		return 0, 4, 6, 4, true
	case BottomRight:
		return 6, 4, 6, 4, true
	case LeftThird:
		return 0, 0, 4, 8, true
	case CenterThird:
		return 4, 0, 4, 8, true
	case RightThird:
		return 8, 0, 4, 8, true
	case LeftTwoThirds:
		return 0, 0, 8, 8, true
	case RightTwoThirds:
		return 4, 0, 8, 8, true
	case Maximize:
		return 0, 0, 12, 8, true
	case AlmostMaximize:
		return 1, 1, 10, 6, true
	case ReasonableSize:
		return 2, 1, 8, 6, true
	case Center:
		return 3, 2, 6, 4, true
	}
	return 0, 0, 0, 0, false
}

// Aliases are the other words each action is found by, beyond its names
// in both languages (copy.md 15.1, 15.2), both languages mixed.
var Aliases = map[string][]string{
	LockScreen:        {"锁屏", "锁定", "lock"},
	Sleep:             {"休眠", "待机", "电脑睡眠", "sleep"},
	SleepDisplays:     {"息屏", "熄屏", "关屏", "屏幕睡眠", "显示器睡眠", "关闭屏幕", "display off"},
	ScreenSaver:       {"屏保", "屏幕保护", "screensaver"},
	Restart:           {"重启", "reboot"},
	ShutDown:          {"关电脑", "shutdown", "power off"},
	LogOut:            {"注销", "登出", "logout", "sign out"},
	ToggleMute:        {"静音", "取消静音", "声音", "mute", "unmute"},
	VolumeUp:          {"音量加", "大声", "声音大", "volume up"},
	VolumeDown:        {"音量减", "小声", "声音小", "volume down"},
	Volume0:           {"音量 0", "volume 0"},
	Volume25:          {"音量 25", "volume 25"},
	Volume50:          {"音量 50", "volume 50"},
	Volume75:          {"音量 75", "volume 75"},
	Volume100:         {"音量 100", "volume 100"},
	ToggleAppearance:  {"深色模式", "浅色模式", "暗黑模式", "夜间模式", "外观", "dark mode", "light mode"},
	ShowDesktop:       {"桌面", "desktop"},
	HideOthers:        {"隐藏其他", "只留当前", "hide others"},
	UnhideAll:         {"全部显示", "取消隐藏", "unhide"},
	QuitAll:           {"全部退出", "关掉所有应用", "quit all"},
	OpenTrash:         {"废纸篓", "回收站", "垃圾桶", "trash"},
	EmptyTrash:        {"清空废纸篓", "清空回收站", "倒垃圾", "empty trash"},
	EjectAll:          {"弹出", "弹出磁盘", "推出", "U 盘", "移动硬盘", "eject"},
	ToggleHiddenFiles: {"隐藏文件", "点文件", "dotfiles", "hidden files"},

	LeftHalf:        {"左半", "左边", "窗口左", "left"},
	RightHalf:       {"右半", "右边", "窗口右", "right"},
	TopHalf:         {"上半", "上边", "top"},
	BottomHalf:      {"下半", "下边", "bottom"},
	TopLeft:         {"左上", "左上角", "1/4"},
	TopRight:        {"右上", "右上角", "1/4"},
	BottomLeft:      {"左下", "左下角", "1/4"},
	BottomRight:     {"右下", "右下角", "1/4"},
	LeftThird:       {"左 1/3", "三分屏", "third"},
	CenterThird:     {"中间 1/3", "三分屏", "third"},
	RightThird:      {"右 1/3", "三分屏", "third"},
	LeftTwoThirds:   {"左 2/3", "two thirds"},
	RightTwoThirds:  {"右 2/3", "two thirds"},
	Maximize:        {"全屏", "铺满", "放到最大", "max", "full"},
	AlmostMaximize:  {"差不多最大", "四周留边", "almost"},
	ReasonableSize:  {"适中", "默认大小", "正常大小", "reasonable"},
	Center:          {"窗口居中", "放中间", "center"},
	MakeLarger:      {"变大", "放大", "larger", "bigger"},
	MakeSmaller:     {"变小", "缩小", "smaller"},
	Restore:         {"恢复", "撤销", "回到原来", "restore", "undo"},
	NextDisplay:     {"下一个屏幕", "下一块显示器", "换屏", "外接屏", "next display", "next screen"},
	PreviousDisplay: {"上一个屏幕", "上一块显示器", "换屏", "previous display"},
}
