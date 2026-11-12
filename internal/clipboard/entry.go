// Package clipboard keeps the clipboard history: a Watcher notices what
// the user copies, a Store keeps it (in memory for search, and on disk),
// and the Store puts entries back on the pasteboard and pastes them into
// the frontmost app.
//
// The Store and its persistence (plain files, in the clear, private to
// the user) are plain Go and work on any platform; reading the system
// pasteboard, the frontmost app, the Accessibility check and the ⌘V
// keystroke are macOS only.
package clipboard

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"html"
	"image/png"
	"net/url"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"
)

// MaxSize is the most an entry may hold, in bytes; larger copies are not
// kept. Files count only their paths, never their contents.
const MaxSize = 20 << 20

// Kind is what an entry holds.
type Kind string

const (
	// KindText is plain text.
	KindText Kind = "text"
	// KindRichText is text with formatting: RTF and/or HTML alongside
	// the plain text.
	KindRichText Kind = "richText"
	// KindLink is text that is a single http or https URL.
	KindLink Kind = "link"
	// KindImage is an image, kept as PNG.
	KindImage Kind = "image"
	// KindFiles is one or more files, kept as their paths.
	KindFiles Kind = "files"
)

// Filter narrows a listing or a search to some kinds (the type filter).
type Filter int

const (
	// FilterAll lists every kind.
	FilterAll Filter = iota
	// FilterText lists plain and rich text.
	FilterText
	// FilterLinks lists links.
	FilterLinks
	// FilterImages lists images.
	FilterImages
	// FilterFiles lists files.
	FilterFiles
)

// Matches reports whether entries of kind k pass the filter.
func (f Filter) Matches(k Kind) bool {
	switch f {
	case FilterText:
		return k == KindText || k == KindRichText
	case FilterLinks:
		return k == KindLink
	case FilterImages:
		return k == KindImage
	case FilterFiles:
		return k == KindFiles
	}
	return true
}

// App is the app something was copied from. The zero App is unknown.
type App struct {
	BundleID string `json:"bundleID,omitempty"`
	Name     string `json:"name,omitempty"`
	// PID is the process, when known; it is not saved.
	PID int `json:"-"`
}

// Content is what a pasteboard holds, as the history keeps it.
type Content struct {
	// Text is the plain text: of text, rich text and links.
	Text string
	// RTF and HTML are the formatted versions of rich text.
	RTF  []byte
	HTML []byte
	// PNG is an image.
	PNG []byte
	// Files are absolute paths of copied files.
	Files []string
}

// Size is the number of bytes the content takes.
func (c Content) Size() int64 {
	n := int64(len(c.Text) + len(c.RTF) + len(c.HTML) + len(c.PNG))
	for _, f := range c.Files {
		n += int64(len(f))
	}
	return n
}

// Kind is what the content is, once normalized: files first, then text
// (a single http/https URL is a link; text with RTF or HTML is rich),
// then an image.
func (c Content) Kind() Kind {
	switch {
	case len(c.Files) > 0:
		return KindFiles
	case strings.TrimSpace(c.Text) != "":
		if IsLink(c.Text) {
			return KindLink
		}
		if len(c.RTF) > 0 || len(c.HTML) > 0 {
			return KindRichText
		}
		return KindText
	case len(c.PNG) > 0:
		return KindImage
	}
	return ""
}

// normalize keeps only what the content's kind uses: files drop the rest,
// text drops images, images drop text. Rich text with no plain text gets
// the text of its HTML. Empty content has kind "".
func normalize(c Content) (Content, Kind) {
	if len(c.Files) > 0 {
		var files []string
		for _, f := range c.Files {
			if f != "" {
				files = append(files, f)
			}
		}
		if len(files) > 0 {
			return Content{Files: files}, KindFiles
		}
	}
	if strings.TrimSpace(c.Text) == "" && len(c.HTML) > 0 {
		c.Text = HTMLText(c.HTML)
	}
	if strings.TrimSpace(c.Text) != "" {
		out := Content{Text: c.Text, RTF: c.RTF, HTML: c.HTML}
		return out, out.Kind()
	}
	if len(c.PNG) > 0 {
		return Content{PNG: c.PNG}, KindImage
	}
	return Content{}, ""
}

// hash identifies content for deduplication: the plain text of text,
// rich text and links (so copying the same words with or without
// formatting keeps one entry, with the latest formatting), the PNG of an
// image, the paths of files.
func hash(c Content, k Kind) string {
	h := sha256.New()
	switch k {
	case KindFiles:
		h.Write([]byte("files\x00"))
		h.Write([]byte(strings.Join(c.Files, "\x00")))
	case KindImage:
		h.Write([]byte("image\x00"))
		h.Write(c.PNG)
	default:
		h.Write([]byte("text\x00"))
		h.Write([]byte(c.Text))
	}
	return hex.EncodeToString(h.Sum(nil))
}

// IsLink reports whether text, trimmed, is a single http or https URL.
func IsLink(text string) bool {
	s := strings.TrimSpace(text)
	if s == "" || strings.ContainsAny(s, " \t\r\n") {
		return false
	}
	u, err := url.Parse(s)
	if err != nil || u.Host == "" {
		return false
	}
	scheme := strings.ToLower(u.Scheme)
	return scheme == "http" || scheme == "https"
}

var (
	htmlDrop  = regexp.MustCompile(`(?is)<(script|style|head)\b.*?</(script|style|head)\s*>`)
	htmlBreak = regexp.MustCompile(`(?i)<(br|/p|/div|/li|/tr|/h[1-6])\b[^>]*>`)
	htmlTag   = regexp.MustCompile(`(?s)<[^>]*>`)
)

// HTMLText is the text of an HTML fragment: tags removed, entities
// decoded, block ends as line breaks. It is for search and display, not a
// faithful rendering.
func HTMLText(b []byte) string {
	s := htmlDrop.ReplaceAllString(string(b), "")
	s = htmlBreak.ReplaceAllString(s, "\n")
	s = htmlTag.ReplaceAllString(s, "")
	s = html.UnescapeString(s)
	return strings.TrimSpace(s)
}

// Entry is one item of the history, without its formatted or image data
// (Store.Content has those).
type Entry struct {
	ID   string
	Kind Kind
	// Text is the plain text of text, rich text and links, and the file
	// paths, one per line, of files; empty for images.
	Text string
	// Files are the paths of files.
	Files []string
	// App is where it was last copied from.
	App App
	// Time is when it was last copied, or put back on the pasteboard.
	Time time.Time
	// Size is the bytes it takes (for files, of their paths).
	Size int64
	// Width and Height are an image's, in pixels.
	Width, Height int
	// Thumbnail is an image's thumbnail, a PNG that fits in
	// ThumbnailSize×ThumbnailSize pixels, made when it was copied. It is
	// shared: do not modify it.
	Thumbnail []byte
	// Pinned entries never expire and list first.
	Pinned bool
}

// Characters is the number of characters of the entry's text, each
// Chinese character counting one.
func (e Entry) Characters() int { return utf8.RuneCountInString(e.Text) }

// imageSize reads the dimensions of a PNG, or 0, 0.
func imageSize(b []byte) (int, int) {
	cfg, err := png.DecodeConfig(bytes.NewReader(b))
	if err != nil {
		return 0, 0
	}
	return cfg.Width, cfg.Height
}

// Section is the group an entry lists under.
type Section int

const (
	// SectionPinned holds the pinned entries, always first.
	SectionPinned Section = iota
	// SectionToday, SectionYesterday and SectionEarlier group the others
	// by the local date they were last copied.
	SectionToday
	SectionYesterday
	SectionEarlier
)

// SectionOf is the group e lists under as of now, by the local dates in
// now's location.
func SectionOf(e Entry, now time.Time) Section {
	if e.Pinned {
		return SectionPinned
	}
	day := func(t time.Time) time.Time {
		y, m, d := t.In(now.Location()).Date()
		return time.Date(y, m, d, 0, 0, 0, 0, now.Location())
	}
	today, copied := day(now), day(e.Time)
	switch {
	case !copied.Before(today):
		return SectionToday
	case copied.Equal(today.AddDate(0, 0, -1)):
		return SectionYesterday
	}
	return SectionEarlier
}

// FirstUnpinned is the index in entries (as Entries or Search list them,
// pinned first) of the newest unpinned entry, the one a fresh view
// selects: 0 when all are pinned, -1 when there are none.
func FirstUnpinned(entries []Entry) int {
	if len(entries) == 0 {
		return -1
	}
	for i, e := range entries {
		if !e.Pinned {
			return i
		}
	}
	return 0
}
