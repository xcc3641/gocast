package main

import (
	_ "embed"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/plugins/updater"
	"github.com/egoist/mygo/plugins/updater/native"

	"gocast/locales"
)

// updateIcon is the update window's icon (design.md 4.5): the app icon
// at 256 px, which assets/icon/make-icon.py draws.
//
//go:embed assets/icon/update-icon.png
var updateIcon []byte

// useUpdater adds MyGo's update window, in native UI (design.md 4.5). It
// checks in the background while 「自动检查更新」 is on (updater.json,
// not settings.json), and 检查更新 on the About page and in the tray
// opens it. Its texts are copy.md 4.5's, without the plugin's
// exclamation marks, in the language Gocast shows: Language is left
// empty, which is App.Locale, the one of Gocast's catalogs that the app
// shows (MYGO_LOCALE included).
func useUpdater() {
	mygo.Use(native.New(updater.Options{
		Icon:    updateIcon,
		Strings: updateStrings(),
	}))
}

// updateStrings are the update window's texts in Gocast's languages,
// from its catalogs.
func updateStrings() map[string]updater.Strings {
	s := map[string]updater.Strings{}
	for _, lang := range []string{"en", "zh-Hans"} {
		m := locales.In(lang)
		s[lang] = updater.Strings{
			Title:              m.UpdateTitle(),
			MenuItem:           m.TrayCheckUpdates(),
			Checking:           m.UpdateChecking(),
			Cancel:             m.UpdateCancel(),
			OK:                 m.UpdateOK(),
			UpToDate:           m.UpdateUpToDate(),
			UpToDateMessage:    m.UpdateUpToDateMessage(),
			Unavailable:        m.UpdateUnavailable(),
			UnavailableMessage: m.UpdateUnavailableMessage(),
			DevelopmentBuild:   m.UpdateDevelopmentBuild(),
			Error:              m.UpdateError(),
			CheckError:         m.UpdateCheckError(),
			InstallError:       m.UpdateInstallError(),
			Available:          m.UpdateAvailable(),
			AvailableMessage:   m.UpdateAvailableMessage(),
			ReleaseNotes:       m.UpdateReleaseNotes(),
			AutomaticDownloads: m.UpdateAutomaticDownloads(),
			Skip:               m.UpdateSkip(),
			RemindLater:        m.UpdateRemindLater(),
			Install:            m.UpdateInstall(),
			Downloading:        m.UpdateDownloading(),
			Progress:           m.UpdateProgress(),
			Megabytes:          m.UpdateMegabytes(),
			Installing:         m.UpdateInstalling(),
			Ready:              m.UpdateReady(),
			ReadyMessage:       m.UpdateReadyMessage(),
			Later:              m.UpdateLater(),
			Relaunch:           m.UpdateRelaunch(),
		}
	}
	return s
}
