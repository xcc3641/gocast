package app

// clipprefs.go is Settings › Clipboard (design.md 4.4, copy.md 6.8):
// recording, how long items are kept, pasting on Return, the apps whose
// copies are not kept, and clearing the history.

import (
	"log"
	"path/filepath"
	"slices"
	"strings"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/plugins/sfsymbols"
	"github.com/egoist/mygo/ui"

	"gocast/internal/apps"
	"gocast/internal/clipboard"
	"gocast/locales"
)

// retentionLabel names how long unpinned items are kept.
func retentionLabel(r clipboard.Retention) string {
	switch r {
	case clipboard.RetainDay:
		return locales.ClipRetentionDay()
	case clipboard.RetainWeek:
		return locales.ClipRetentionWeek()
	case clipboard.Retain3Months:
		return locales.ClipRetention3Months()
	case clipboard.RetainForever:
		return locales.ClipRetentionForever()
	}
	return locales.ClipRetentionMonth()
}

// loadClipboard reads the page's settings, as it shows.
func (p *prefsWindow) loadClipboard() {
	s := p.g.clip
	p.clipKeep = s.recording()
	p.clipPaste = p.g.settings.PasteOnReturn()
	p.clipRetention = retentionLabel(s.retention())
	if !p.g.launcher.loaded {
		p.g.launcher.load() // the apps, to show the ignored ones
	}
}

// ignoredApp is an app whose copies are not kept, installed on this Mac.
type ignoredApp struct{ id, name, path string }

// installedIgnored are the ignored apps installed here, in the list's
// order: those not installed stay in the list, unshown (design.md 4.4).
func (p *prefsWindow) installedIgnored() []ignoredApp {
	var out []ignoredApp
	for _, id := range p.g.clip.ignoredApps() {
		path := appPathOf(p.g.launcher, id)
		if path == "" {
			continue
		}
		name := strings.TrimSuffix(filepath.Base(path), ".app")
		for _, a := range p.g.launcher.apps {
			if a.ID == id {
				name = a.Name
			}
		}
		out = append(out, ignoredApp{id, name, path})
	}
	return out
}

func (p *prefsWindow) clipboardPage(c *ui.Context, k tokens) {
	s := p.g.clip
	pageTitle(c, k, locales.PageClipboard())
	newGroup(c, k, func(g *group) {
		g.row(c, k, locales.ClipKeep(), locales.ClipKeepHint(), k.textSecondary, func() {
			if ui.Switch(c, &p.clipKeep).Label(locales.ClipKeep()).Changed() {
				s.setRecording(p.clipKeep)
			}
		})
		g.row(c, k, locales.ClipRetention(), locales.ClipRetentionHint(), k.textSecondary, func() {
			labels := make([]string, len(clipboard.Retentions))
			for i, r := range clipboard.Retentions {
				labels[i] = retentionLabel(r)
			}
			if ui.Select(c, &p.clipRetention, labels).Width(120).Label(locales.ClipRetention()).Changed() {
				if i := slices.Index(labels, p.clipRetention); i >= 0 {
					s.setRetention(clipboard.Retentions[i])
				}
			}
		})
		// Pasting on Return: while Gocast may not press ⌘V, the
		// description says so, with a button to System Settings.
		groupDivider(c, k)
		g.rows++
		noAX := p.clipPaste && !s.trusted()
		ui.Row(c).MinHeight(44).Padding(8, 14, 8, 0).Gap(12).AlignItems(ui.Center).Children(func() {
			ui.Column(c).Grow(1).Shrink(1).Gap(2).Children(func() {
				ui.Text(c, locales.ClipPasteOnEnter()).FontSize(13).TextColor(k.text)
				if noAX {
					ui.Text(c, locales.ClipNoAXHint()).FontSize(12).TextColor(k.warning)
					ui.Row(c).Margin(6, 0, 0, 0).Children(func() { // 8 under the text with the gap
						ui.Button(c, locales.ClipOpenSettings()).Height(24).FontSize(12).OnClick(s.openAccessibility)
					})
				} else {
					ui.Text(c, locales.ClipPasteOnEnterHint()).FontSize(12).TextColor(k.textSecondary)
				}
			})
			if ui.Switch(c, &p.clipPaste).Label(locales.ClipPasteOnEnter()).Changed() {
				s.setPasteOnReturn(p.clipPaste)
			}
		})
	})
	groupTitle(c, k, locales.ClipIgnoredTitle(), locales.ClipIgnoredHint())
	p.ignoredGroup(c, k)
	canClear := s.store != nil && s.store.CanClear()
	ui.Row(c).Children(func() {
		ui.Button(c, locales.ClipClear()).TextColor(k.danger).Disabled(!canClear).OnClick(func() { p.clipConfirm = true })
	})
}

// ignoredGroup lists the ignored apps installed here, one chosen at a
// time, over a bar adding and removing them.
func (p *prefsWindow) ignoredGroup(c *ui.Context, k tokens) {
	list := p.installedIgnored()
	if !slices.ContainsFunc(list, func(a ignoredApp) bool { return a.id == p.clipIgnoredSel }) {
		p.clipIgnoredSel = ""
	}
	ui.Column(c).Background(k.groupFill).Radius(10).Padding(6, groupInset, 0, groupInset).Margin(0, 0, 20, 0).Children(func() {
		ui.Column(c).Role(ui.RoleList).Label(locales.ClipIgnoredTitle()).Children(func() {
			for _, a := range list {
				id := a.id
				on := id == p.clipIgnoredSel
				c.Key(id)
				r := ui.Row(c).Height(36).Padding(0, 8).Margin(0, -8).Radius(6).Gap(10).AlignItems(ui.Center).
					Role(ui.RoleListItem).AccessSelected(on).Label(a.name).OnClick(func() { p.clipIgnoredSel = id })
				if on {
					r.Background(k.selection)
				}
				r.Children(func() {
					if b := p.g.launcher.clip.fileIcon(a.path, 20); b != nil {
						ui.Image(c, b).Size(20, 20).Shrink(0).AccessHidden(true)
					}
					ui.Text(c, a.name).FontSize(13).TextColor(k.text).SingleLine().Shrink(1)
				})
			}
		})
		if len(list) > 0 {
			ui.Box(c).Height(1).Background(k.hairline).Margin(6, -groupInset, 0, -groupInset).Shrink(0)
		}
		ui.Row(c).Height(32).Gap(2).AlignItems(ui.Center).Children(func() {
			p.barButton(c, k, "plus", locales.ClipAddApp(), false, p.addIgnoredApp)
			p.barButton(c, k, "minus", locales.ClipRemoveApp(), p.clipIgnoredSel == "", func() {
				ids := slices.DeleteFunc(p.g.clip.ignoredApps(), func(id string) bool { return id == p.clipIgnoredSel })
				p.g.clip.setIgnoredApps(ids)
				p.clipIgnoredSel = ""
			})
		})
	})
}

// barButton is a borderless button with a symbol, 24 by 24, of the bar
// under a list.
func (p *prefsWindow) barButton(c *ui.Context, k tokens, symbol, label string, disabled bool, act func()) {
	b := ui.Box(c).Size(24, 24).Radius(5).Center().Shrink(0).Focusable().Role(ui.RoleButton).Label(label).Tooltip(label)
	color := k.text
	if disabled {
		color = k.text.Alpha(0.3)
		b.AccessDisabled(true)
	} else {
		if b.Pressed() {
			b.Background(k.selection)
		}
		b.OnClick(act)
	}
	b.Children(func() { sfsymbols.Symbol(c, symbol).Size(12).Color(color) })
}

// addIgnoredApp asks for an app, in the Applications folder, whose copies
// the history is not to keep.
func (p *prefsWindow) addIgnoredApp() {
	paths, err := mygo.Dialog.Open(mygo.OpenDialogOptions{
		Parent: p.win, DefaultPath: "/Applications", ButtonLabel: locales.ClipAddButton(),
		Filters: []mygo.FileFilter{{Name: locales.ClipAddApp(), Extensions: []string{"app"}}},
	})
	if err != nil {
		log.Print(err)
		return
	}
	if len(paths) == 0 {
		return
	}
	id := apps.BundleID(paths[0])
	if id == "" {
		return
	}
	ids := p.g.clip.ignoredApps()
	if !slices.Contains(ids, id) {
		p.g.clip.setIgnoredApps(append(ids, id))
	}
	p.clipIgnoredSel = id
	p.win.Invalidate()
}

// clearConfirmation asks before clearing the history (design.md 2.8.8,
// copy.md 6.9): Cancel the default, Clear in danger red.
func (p *prefsWindow) clearConfirmation(c *ui.Context, k tokens) {
	if !p.clipConfirm {
		return
	}
	s := p.g.clip
	body := locales.ClipClearBody()
	if s.store != nil && s.store.Unreadable() {
		body = locales.ClipClearBodyUnreadable()
	}
	ui.Modal(c, &p.clipConfirm, func() {
		ui.Column(c).Width(340).Padding(20).Gap(6).Role(ui.RoleAlertDialog).Label(locales.ClipClearTitle()).Children(func() {
			ui.Text(c, locales.ClipClearTitle()).FontSize(14).FontWeight(600).TextColor(k.text).Role(ui.RoleHeading)
			ui.Text(c, body).FontSize(13).TextColor(k.textSecondary)
			ui.Row(c).Margin(10, 0, 0, 0).Gap(8).Children(func() {
				ui.Spacer(c)
				ui.Button(c, locales.Cancel()).AutoFocus().OnClick(func() { p.clipConfirm = false })
				ui.Button(c, locales.ClipClearConfirm()).TextColor(k.danger).OnClick(func() {
					p.clipConfirm = false
					s.clear()
				})
			})
		})
	})
}
