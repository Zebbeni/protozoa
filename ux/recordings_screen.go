package ux

import (
	"fmt"
	"image/color"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/hajimehoshi/ebiten/v2/text"

	"github.com/Zebbeni/protozoa/checkpoint"
	c "github.com/Zebbeni/protozoa/config"
	r "github.com/Zebbeni/protozoa/resources"
)

type RecordingsResult int

const (
	// RecordingsNone means the browser is still open.
	RecordingsNone RecordingsResult = iota
	RecordingsBack
	// RecordingsOpen means Chosen() holds the path to play.
	RecordingsOpen
)

const (
	recRowH       = 40
	recRowGap     = 6
	recListW      = 520
	recTitleH     = 56
	recFooterH    = 54
	recDeleteW    = 70
	recPad        = 16
	recVisibleMax = 9
)

type RecordingsScreen struct {
	// dir is where recordings are read from and deleted in.
	dir    string
	items  []checkpoint.SavedRecording
	chosen string
	// confirmDelete is the row index armed for deletion, or -1. The first click arms, the second deletes.
	confirmDelete int
	message       string
	messageErr    bool
	scroll        int
}

func NewRecordingsScreen() *RecordingsScreen {
	return newRecordingsScreenIn(checkpoint.RecordingsDir)
}

func newRecordingsScreenIn(dir string) *RecordingsScreen {
	s := &RecordingsScreen{dir: dir, confirmDelete: -1}
	s.reload()
	return s
}

func (s *RecordingsScreen) reload() {
	s.items = checkpoint.SavedRecordings(s.dir)
	s.confirmDelete = -1
	if s.scroll > s.maxScroll() {
		s.scroll = s.maxScroll()
	}
}

// Chosen is the recording the user picked, valid after Update returns RecordingsOpen.
func (s *RecordingsScreen) Chosen() string { return s.chosen }

func (s *RecordingsScreen) maxScroll() int {
	if len(s.items) <= recVisibleMax {
		return 0
	}
	return len(s.items) - recVisibleMax
}

// visible is the slice of rows on screen, and the index the first one sits at in items.
func (s *RecordingsScreen) visible() ([]checkpoint.SavedRecording, int) {
	lo := s.scroll
	hi := min(lo+recVisibleMax, len(s.items))
	return s.items[lo:hi], lo
}

func (s *RecordingsScreen) Update() RecordingsResult {
	if inpututil.IsKeyJustPressed(ebiten.KeyEscape) {
		return RecordingsBack
	}
	if _, wy := ebiten.Wheel(); wy != 0 {
		s.scroll = max(0, min(s.maxScroll(), s.scroll-int(wy)))
	}
	if !inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
		return RecordingsNone
	}
	mx, my := ebiten.CursorPosition()

	if hitRect(mx, my, recBackRect()) {
		return RecordingsBack
	}

	rows, offset := s.visible()
	for i := range rows {
		row := recRowRect(i)
		if !hitRect(mx, my, row) {
			continue
		}
		idx := offset + i
		if hitRect(mx, my, recDeleteRect(i)) {
			s.deleteAt(idx)
			return RecordingsNone
		}
		s.confirmDelete = -1
		s.chosen = s.items[idx].Path
		return RecordingsOpen
	}

	// A click anywhere else disarms a waiting confirm.
	s.confirmDelete = -1
	return RecordingsNone
}

// deleteAt removes a recording after arming, then reloads the list.
func (s *RecordingsScreen) deleteAt(i int) {
	if i < 0 || i >= len(s.items) {
		return
	}
	if s.confirmDelete != i {
		s.confirmDelete = i
		s.setMessage("click again to delete "+s.items[i].Name, false)
		return
	}
	name := s.items[i].Name
	if err := checkpoint.DeleteRecording(s.dir, name); err != nil {
		s.setMessage(err.Error(), true)
		s.confirmDelete = -1
		return
	}
	s.reload()
	s.setMessage("deleted "+name, false)
}

func (s *RecordingsScreen) setMessage(msg string, isErr bool) {
	s.message, s.messageErr = msg, isErr
}

func (s *RecordingsScreen) Draw(screen *ebiten.Image) {
	fillThemeBackground(screen)

	title := "SAVED RECORDINGS"
	tb := boundString(r.FontSourceCodePro12, title)
	text.Draw(screen, title, r.FontSourceCodePro12,
		(c.ScreenWidth()-tb.Dx())/2, recTitleH, themedForeground())

	if len(s.items) == 0 {
		// An empty list is the normal state until the first save.
		msg := "No saved recordings yet."
		how := "Open a replay, then MENU > Save Recording As..."
		mb := boundString(r.FontSourceCodePro12, msg)
		hb := boundString(r.FontSourceCodePro10, how)
		y := c.ScreenHeight()/2 - 20
		text.Draw(screen, msg, r.FontSourceCodePro12, (c.ScreenWidth()-mb.Dx())/2, y, themedForeground())
		text.Draw(screen, how, r.FontSourceCodePro10, (c.ScreenWidth()-hb.Dx())/2, y+24, themedMuted())
		s.drawFooter(screen)
		return
	}

	mx, my := ebiten.CursorPosition()
	pressed := ebiten.IsMouseButtonPressed(ebiten.MouseButtonLeft)
	rows, offset := s.visible()
	for i, rec := range rows {
		row := recRowRect(i)
		hovered := hitRect(mx, my, row)
		fill := themedControlDim()
		if hovered {
			fill = themedControlFill()
		}
		fillRect(screen, row, fill)

		text.Draw(screen, rec.Name, r.FontSourceCodePro12,
			row.Min.X+10, row.Min.Y+17, themedValue())
		sub := fmt.Sprintf("%s  ·  %s", formatRecSize(rec.Size), humanAge(rec.Mod))
		text.Draw(screen, sub, r.FontSourceCodePro8,
			row.Min.X+10, row.Min.Y+31, themedMuted())

		del := recDeleteRect(i)
		label := "Delete"
		if s.confirmDelete == offset+i {
			label = "Sure?"
		}
		drawMenuButton(screen, del.Min.X, del.Min.Y, del.Dx(), del.Dy(),
			label, hitRect(mx, my, del), hitRect(mx, my, del) && pressed, false)
	}

	if s.maxScroll() > 0 {
		note := fmt.Sprintf("%d-%d of %d  (scroll)", offset+1, offset+len(rows), len(s.items))
		nb := boundString(r.FontSourceCodePro8, note)
		text.Draw(screen, note, r.FontSourceCodePro8,
			(c.ScreenWidth()-nb.Dx())/2, recRowRect(len(rows)-1).Max.Y+16, themedMuted())
	}
	s.drawFooter(screen)
}

func (s *RecordingsScreen) drawFooter(screen *ebiten.Image) {
	mx, my := ebiten.CursorPosition()
	pressed := ebiten.IsMouseButtonPressed(ebiten.MouseButtonLeft)
	back := recBackRect()
	drawMenuButton(screen, back.Min.X, back.Min.Y, back.Dx(), back.Dy(),
		"Back", hitRect(mx, my, back), hitRect(mx, my, back) && pressed, false)

	if s.message == "" {
		return
	}
	col := color.Color(themedForegroundDim())
	if s.messageErr {
		col = themedBad()
	}
	text.Draw(screen, s.message, r.FontSourceCodePro8,
		(c.ScreenWidth()-recListW)/2, back.Min.Y-10, col)
}

func recListX() int { return (c.ScreenWidth() - recListW) / 2 }

func recRowRect(i int) popupRectT {
	return newRect(recListX(), recTitleH+recPad+i*(recRowH+recRowGap), recListW, recRowH)
}

func recDeleteRect(i int) popupRectT {
	row := recRowRect(i)
	return newRect(row.Max.X-recPad/2-recDeleteW, row.Min.Y+(recRowH-26)/2, recDeleteW, 26)
}

func recBackRect() popupRectT {
	return newRect(recListX(), c.ScreenHeight()-recFooterH, 120, 34)
}

// formatRecSize is the same scale the replay footer reports file sizes on.
func formatRecSize(n int64) string {
	const (
		kb = 1024
		mb = kb * 1024
		gb = mb * 1024
	)
	switch {
	case n >= gb:
		return fmt.Sprintf("%.2f GB", float64(n)/float64(gb))
	case n >= mb:
		return fmt.Sprintf("%.1f MB", float64(n)/float64(mb))
	case n >= kb:
		return fmt.Sprintf("%.0f KB", float64(n)/float64(kb))
	default:
		return fmt.Sprintf("%d B", n)
	}
}

// humanAge is how long ago a recording was saved, in the coarsest unit that still says something.
func humanAge(t time.Time) string {
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	case d < 365*24*time.Hour:
		return fmt.Sprintf("%dd ago", int(d.Hours()/24))
	}
	return t.Format("2006-01-02")
}
