//go:build darwin

package clipboard

import (
	"slices"
	"strings"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/transfer"
)

// rtfFormat is RTF's MIME name, which MyGo maps to public.rtf.
const rtfFormat transfer.Format = "text/rtf"

// systemPasteboard is the general pasteboard: MyGo's Clipboard reads and
// writes it (on the main thread, as MyGo does); the change count comes
// straight from NSPasteboard, which MyGo does not offer.
type systemPasteboard struct{}

// SystemPasteboard is the general pasteboard. Its Types, Read and Write
// go through MyGo's Clipboard, so they work once the app runs
// (App.WhenReady), from any goroutine.
func SystemPasteboard() Pasteboard { return systemPasteboard{} }

func (systemPasteboard) ChangeCount() int { return changeCount() }

func (systemPasteboard) Types() []string { return mygo.Clipboard.AvailableFormats() }

func (systemPasteboard) Read() (Content, error) {
	formats := mygo.Clipboard.Formats()
	has := func(f transfer.Format) bool { return slices.Contains(formats, f) }
	if has(transfer.FileList) {
		if paths, err := mygo.Clipboard.ReadFiles(); err == nil && len(paths) > 0 {
			return Content{Files: paths}, nil
		}
	}
	var c Content
	var want []transfer.Format
	for _, f := range []transfer.Format{transfer.Text, rtfFormat, transfer.HTML} {
		if has(f) {
			want = append(want, f)
		}
	}
	if len(want) > 0 {
		d, err := mygo.Clipboard.Read(want...)
		if err != nil {
			return Content{}, err
		}
		if b, err := d.Read(transfer.Text); err == nil {
			c.Text = string(b)
		}
		if b, err := d.Read(rtfFormat); err == nil {
			c.RTF = b
		}
		if b, err := d.Read(transfer.HTML); err == nil {
			c.HTML = b
		}
	}
	if c.Text == "" && has(transfer.URIList) {
		if d, err := mygo.Clipboard.Read(transfer.URIList); err == nil {
			if urls, err := d.URLs(); err == nil {
				c.Text = strings.Join(urls, "\n")
			}
		}
	}
	// An image only when there is no text: apps that copy text often add
	// a picture of it.
	if _, k := normalize(c); k == "" && has(transfer.PNG) {
		b, err := mygo.Clipboard.ReadFormat(transfer.PNG)
		if err != nil {
			return Content{}, err
		}
		c.PNG = b
	}
	return c, nil
}

func (systemPasteboard) Write(c Content) error {
	d, err := transferData(c)
	if err != nil {
		return err
	}
	if err := mygo.Clipboard.Write(d); err != nil {
		return err
	}
	// Hand the data to the pasteboard now, so that pasting does not wait
	// on Gocast's main thread to provide it. The data is on the
	// pasteboard either way; a failed flush only delays it.
	_ = mygo.Clipboard.Flush()
	return nil
}

// transferData is c as MyGo transfer data: files as file URLs, one item
// each; text as one item with its RTF and HTML first (the richest the
// receiver takes wins), the plain text, and for a link a URL; an image as
// PNG.
func transferData(c Content) (transfer.Data, error) {
	switch {
	case len(c.Files) > 0:
		return transfer.FileData(c.Files...)
	case c.Text != "":
		var reps []transfer.Representation
		if len(c.RTF) > 0 {
			reps = append(reps, transfer.Bytes(rtfFormat, c.RTF))
		}
		if len(c.HTML) > 0 {
			reps = append(reps, transfer.Bytes(transfer.HTML, c.HTML))
		}
		reps = append(reps, transfer.Bytes(transfer.Text, []byte(c.Text)))
		if IsLink(c.Text) {
			reps = append(reps, transfer.Bytes(transfer.URIList, []byte(strings.TrimSpace(c.Text))))
		}
		return transfer.New(transfer.NewItem(reps...)), nil
	case len(c.PNG) > 0:
		return transfer.New(transfer.NewItem(transfer.Bytes(transfer.PNG, c.PNG))), nil
	}
	return transfer.Data{}, ErrEmpty
}
