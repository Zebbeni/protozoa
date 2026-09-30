package checkpoint

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func nowPlus(name string) time.Time {
	if name == "newer" {
		return time.Now().Add(time.Hour)
	}
	return time.Now()
}

func writeSrc(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "protozoa_last.pzr")
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestSaveRecordingCopiesTheBytes(t *testing.T) {
	const body = "PZR\x00not-really-a-recording-but-bytes-are-bytes"
	src := writeSrc(t, body)
	dir := filepath.Join(t.TempDir(), RecordingsDir)

	path, err := SaveRecordingAs(src, dir, "My Best Run")
	if err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != body {
		t.Errorf("saved %q, source was %q", got, body)
	}

	// Independent of the source: overwriting the original, as the next run does, must not touch the copy.
	if err := os.WriteFile(src, []byte("a whole new run"), 0o644); err != nil {
		t.Fatal(err)
	}
	again, _ := os.ReadFile(path)
	if string(again) != body {
		t.Errorf("the copy changed when the source was overwritten: %q", again)
	}
}

func TestSaveRecordingRefusesToOverwrite(t *testing.T) {
	src := writeSrc(t, "first")
	dir := filepath.Join(t.TempDir(), RecordingsDir)

	first, err := SaveRecordingAs(src, dir, "keeper")
	if err != nil {
		t.Fatal(err)
	}

	second := writeSrc(t, "second")
	_, err = SaveRecordingAs(second, dir, "keeper")
	if err == nil {
		t.Fatal("saving over an existing recording succeeded")
	}
	if !strings.Contains(err.Error(), "keeper") {
		t.Errorf("error %q doesn't name the file the user has to rename", err)
	}
	if body, _ := os.ReadFile(first); string(body) != "first" {
		t.Errorf("the existing recording was clobbered anyway: %q", body)
	}
}

func TestRecordingFileNameSlugsLikeDesigns(t *testing.T) {
	cases := map[string]string{
		"My Best Run":    "my-best-run.pzr",
		"  spaced  out ": "spaced--out.pzr",
		"Seed_12345":     "seed-12345.pzr",
		"!!!":            "recording.pzr",
		"":               "recording.pzr",
		"UPPER/lower":    "upperlower.pzr",
	}
	for in, want := range cases {
		if got := RecordingFileName(in); got != want {
			t.Errorf("RecordingFileName(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestSaveRecordingRejectsAnEmptySource(t *testing.T) {
	src := writeSrc(t, "")
	dir := filepath.Join(t.TempDir(), RecordingsDir)

	if _, err := SaveRecordingAs(src, dir, "empty"); err == nil {
		t.Error("saving an empty recording succeeded")
	}
	if _, err := os.Stat(filepath.Join(dir, "empty.pzr")); err == nil {
		t.Error("an empty recording was written anyway")
	}
}

func TestSaveRecordingReadsAMemFile(t *testing.T) {
	const path = "mem://replay.pzr"
	m := registerMemFile(path)
	defer delete(memRegistry, path)
	if _, err := m.View().Write([]byte("in-memory recording")); err != nil {
		t.Fatal(err)
	}

	dir := filepath.Join(t.TempDir(), RecordingsDir)
	out, err := SaveRecordingAs(path, dir, "from-memory")
	if err != nil {
		t.Fatal(err)
	}
	if body, _ := os.ReadFile(out); string(body) != "in-memory recording" {
		t.Errorf("saved %q from the MemFile", body)
	}
}

func TestSavedRecordingsListsNewestFirst(t *testing.T) {
	dir := t.TempDir()
	if got := SavedRecordings(filepath.Join(dir, "nothing-here")); got != nil {
		t.Errorf("a missing directory listed %v, want nothing", got)
	}

	src := writeSrc(t, "body")
	for _, name := range []string{"older", "newer"} {
		if _, err := SaveRecordingAs(src, dir, name); err != nil {
			t.Fatal(err)
		}
		os.Chtimes(filepath.Join(dir, name+".pzr"), nowPlus(name), nowPlus(name))
	}
	os.WriteFile(filepath.Join(dir, "notes.txt"), []byte("x"), 0o644)

	got := SavedRecordings(dir)
	if len(got) != 2 || got[0].Name != "newer" || got[1].Name != "older" {
		t.Errorf("listed %v, want [newer older]", got)
	}
	// The browser shows a size and opens the path, so both have to be filled in rather than left zero.
	for _, rec := range got {
		if rec.Size != int64(len("body")) {
			t.Errorf("%s reports size %d, file holds %d bytes", rec.Name, rec.Size, len("body"))
		}
		if _, err := os.Stat(rec.Path); err != nil {
			t.Errorf("%s has an unopenable path %q", rec.Name, rec.Path)
		}
	}

	// Deleting one takes it out of the listing.
	if err := DeleteRecording(dir, "older"); err != nil {
		t.Fatal(err)
	}
	if err := DeleteRecording(dir, "older"); err != nil {
		t.Errorf("deleting a missing recording errored: %v", err)
	}
	if left := SavedRecordings(dir); len(left) != 1 || left[0].Name != "newer" {
		t.Errorf("after delete, listed %v", left)
	}
}
