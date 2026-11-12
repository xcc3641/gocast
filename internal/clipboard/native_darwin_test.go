//go:build darwin

package clipboard

import (
	"errors"
	"os"
	"testing"

	"github.com/egoist/mygo/transfer"
)

// TestNativeSymbols loads the frameworks and functions the package calls,
// without calling any that change state.
func TestNativeSymbols(t *testing.T) {
	if !loadCF() {
		t.Error("CoreFoundation")
	}
	if !loadSecurity() {
		t.Error("Security")
	}
	if !loadAX() || ax.withOptions == nil || ax.prompt == 0 {
		t.Error("ApplicationServices")
	}
	if !loadCG() {
		t.Error("CoreGraphics")
	}
	_ = AccessibilityTrusted() // a read-only check
}

func TestTransferData(t *testing.T) {
	formats := func(c Content) []transfer.Format {
		t.Helper()
		d, err := transferData(c)
		if err != nil {
			t.Fatal(err)
		}
		return d.Formats()
	}
	check := func(c Content, want ...transfer.Format) {
		t.Helper()
		got := formats(c)
		if len(got) != len(want) {
			t.Errorf("%+v: %v, want %v", c, got, want)
			return
		}
		for i := range want {
			if got[i] != want[i] {
				t.Errorf("%+v: %v, want %v", c, got, want)
			}
		}
	}
	check(Content{Text: "plain"}, transfer.Text)
	check(Content{Text: "rich", RTF: []byte("{\\rtf1 x}"), HTML: []byte("<b>x</b>")}, rtfFormat, transfer.HTML, transfer.Text)
	check(Content{Text: "https://example.com/a"}, transfer.Text, transfer.URIList)
	check(Content{PNG: []byte{1}}, transfer.PNG)
	check(Content{Files: []string{"/tmp/a", "/tmp/b"}}, transfer.FileList, transfer.URIList)
	d, _ := transferData(Content{Files: []string{"/tmp/a b", "/tmp/c"}})
	if files, err := d.Files(); err != nil || len(files) != 2 || files[0] != "/tmp/a b" {
		t.Errorf("files %v %v", files, err)
	}
	if _, err := transferData(Content{}); !errors.Is(err, ErrEmpty) {
		t.Errorf("empty: %v", err)
	}
}

// TestSystemPasteboardReadOnly reads the change count and the frontmost
// app; it writes nothing. Run with GOCAST_PASTEBOARD_TEST=1.
func TestSystemPasteboardReadOnly(t *testing.T) {
	if os.Getenv("GOCAST_PASTEBOARD_TEST") != "1" {
		t.Skip("set GOCAST_PASTEBOARD_TEST=1 to read the system pasteboard")
	}
	pb := SystemPasteboard()
	n := pb.ChangeCount()
	if n < 0 || pb.ChangeCount() != n {
		t.Errorf("change count %d", n)
	}
	app := FrontmostApp()
	if app.BundleID == "" || app.PID <= 0 {
		t.Errorf("frontmost %+v", app)
	}
	t.Logf("change count %d, frontmost %+v", n, app)
}

// TestKeychain stores, reads and deletes a key in the login Keychain,
// under a service of its own. Run with GOCAST_KEYCHAIN_TEST=1.
func TestKeychain(t *testing.T) {
	if os.Getenv("GOCAST_KEYCHAIN_TEST") != "1" {
		t.Skip("set GOCAST_KEYCHAIN_TEST=1 to use the Keychain")
	}
	k := &Keychain{Service: KeychainService + ".test", Account: "test-key"}
	t.Cleanup(func() {
		if err := k.Delete(); err != nil {
			t.Errorf("cleanup: %v", err)
		}
	})
	if _, err := k.Load(); !errors.Is(err, ErrNoKey) {
		t.Fatalf("Load before Save: %v", err)
	}
	key, _ := newKey()
	if err := k.Save(key); err != nil {
		t.Fatal(err)
	}
	got, err := k.Load()
	if err != nil || string(got) != string(key) {
		t.Fatalf("Load: %v", err)
	}
	if err := k.Delete(); err != nil {
		t.Fatal(err)
	}
	if _, err := k.Load(); !errors.Is(err, ErrNoKey) {
		t.Errorf("Load after Delete: %v", err)
	}
}
