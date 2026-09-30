package ux

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Zebbeni/protozoa/checkpoint"
	r "github.com/Zebbeni/protozoa/resources"
)

// recordingsIn writes n stand-in recordings into a temp dir and returns a browser pointed at it.
func recordingsIn(t *testing.T, names ...string) (*RecordingsScreen, string) {
	t.Helper()
	dir := t.TempDir()
	for _, name := range names {
		path := filepath.Join(dir, checkpoint.RecordingFileName(name))
		if err := os.WriteFile(path, []byte("recording:"+name), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return newRecordingsScreenIn(dir), dir
}

func TestRecordingsDeleteAsksTwice(t *testing.T) {
	s, dir := recordingsIn(t, "keeper")
	if len(s.items) != 1 {
		t.Fatalf("listed %d recordings, want 1", len(s.items))
	}

	s.deleteAt(0)
	if _, err := os.Stat(filepath.Join(dir, "keeper.pzr")); err != nil {
		t.Fatal("the first click deleted the recording outright")
	}
	if s.confirmDelete != 0 {
		t.Errorf("first click left confirmDelete at %d, want 0", s.confirmDelete)
	}
	if !strings.Contains(s.message, "keeper") {
		t.Errorf("the prompt %q doesn't name what is about to go", s.message)
	}

	s.deleteAt(0)
	if _, err := os.Stat(filepath.Join(dir, "keeper.pzr")); !os.IsNotExist(err) {
		t.Error("the second click didn't delete the recording")
	}
	if len(s.items) != 0 {
		t.Errorf("the list still holds %d rows after deleting the only one", len(s.items))
	}
}

func TestRecordingsConfirmDisarms(t *testing.T) {
	s, dir := recordingsIn(t, "one", "two")

	s.deleteAt(0)
	if s.confirmDelete != 0 {
		t.Fatalf("arming failed: confirmDelete is %d", s.confirmDelete)
	}

	// Arming a different row re-arms rather than deleting the first.
	s.deleteAt(1)
	if s.confirmDelete != 1 {
		t.Errorf("clicking another row left confirmDelete at %d", s.confirmDelete)
	}
	for _, name := range []string{"one", "two"} {
		if _, err := os.Stat(filepath.Join(dir, name+".pzr")); err != nil {
			t.Errorf("%s was deleted by an unconfirmed click", name)
		}
	}

	// reload disarms too, so a list refreshed under a waiting confirm can't delete whatever landed at that index.
	s.deleteAt(1)
	s.deleteAt(0)
	s.reload()
	if s.confirmDelete != -1 {
		t.Errorf("reload left confirmDelete armed at %d", s.confirmDelete)
	}
}

func TestRecordingsDeleteOutOfRangeIsSafe(t *testing.T) {
	s, _ := recordingsIn(t, "only")
	s.deleteAt(-1)
	s.deleteAt(5)
	if len(s.items) != 1 {
		t.Errorf("an out-of-range delete changed the list to %d rows", len(s.items))
	}
}

func TestRecordingsEmptyDirIsNotAnError(t *testing.T) {
	s := newRecordingsScreenIn(filepath.Join(t.TempDir(), "never-created"))
	if len(s.items) != 0 {
		t.Errorf("a missing directory listed %d recordings", len(s.items))
	}
	if s.maxScroll() != 0 {
		t.Errorf("an empty list scrolls to %d", s.maxScroll())
	}
	rows, offset := s.visible()
	if len(rows) != 0 || offset != 0 {
		t.Errorf("an empty list has %d visible rows at offset %d", len(rows), offset)
	}
}

func TestRecordingsScrollWindow(t *testing.T) {
	names := make([]string, 0, recVisibleMax+3)
	for i := 0; i < recVisibleMax+3; i++ {
		names = append(names, string(rune('a'+i)))
	}
	s, _ := recordingsIn(t, names...)

	if got := s.maxScroll(); got != 3 {
		t.Errorf("maxScroll is %d, want 3", got)
	}
	rows, offset := s.visible()
	if len(rows) != recVisibleMax || offset != 0 {
		t.Errorf("unscrolled window is %d rows at %d", len(rows), offset)
	}

	s.scroll = s.maxScroll()
	rows, offset = s.visible()
	if len(rows) != recVisibleMax || offset != 3 {
		t.Errorf("scrolled window is %d rows at %d", len(rows), offset)
	}

	// Deleting down to a short list must pull the scroll back, or the window starts past the end.
	for i := 0; i < 5; i++ {
		s.deleteAt(0)
		s.deleteAt(0)
	}
	if s.scroll > s.maxScroll() {
		t.Errorf("scroll %d left past maxScroll %d after deletes", s.scroll, s.maxScroll())
	}
	rows, offset = s.visible()
	if offset+len(rows) > len(s.items) {
		t.Errorf("window %d..%d runs past %d items", offset, offset+len(rows), len(s.items))
	}
}

func TestRecordingsRowLayout(t *testing.T) {
	loadKeyGlobals(t)

	for i := 0; i < recVisibleMax; i++ {
		row, del := recRowRect(i), recDeleteRect(i)
		if del.Min.X < row.Min.X || del.Max.X > row.Max.X ||
			del.Min.Y < row.Min.Y || del.Max.Y > row.Max.Y {
			t.Errorf("row %d: delete button %v is outside its row %v", i, del, row)
		}
		if i > 0 && recRowRect(i-1).Max.Y >= row.Min.Y {
			t.Errorf("row %d overlaps the one above it", i)
		}
	}
	last := recRowRect(recVisibleMax - 1)
	if last.Max.Y >= recBackRect().Min.Y {
		t.Errorf("the last row ends at %d, the Back button starts at %d", last.Max.Y, recBackRect().Min.Y)
	}
}

func TestRecordingsEmptyStateFitsTheScreen(t *testing.T) {
	loadKeyGlobals(t)
	r.UseDirAssets("..")
	r.Init()

	for _, line := range []string{
		"SAVED RECORDINGS",
		"No saved recordings yet.",
		"Open a replay, then MENU > Save Recording As...",
	} {
		if w := boundString(r.FontSourceCodePro12, line).Dx(); w > recListW {
			t.Errorf("%q is %dpx, wider than the %dpx list it is centred on", line, w, recListW)
		}
	}
}

func TestFormatRecSizeAndAge(t *testing.T) {
	cases := map[int64]string{
		512:              "512 B",
		2048:             "2 KB",
		5 << 20:          "5.0 MB",
		int64(3) << 30:   "3.00 GB",
		int64(250) << 20: "250.0 MB",
	}
	for in, want := range cases {
		if got := formatRecSize(in); got != want {
			t.Errorf("formatRecSize(%d) = %q, want %q", in, got, want)
		}
	}
}
