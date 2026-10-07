package checkpoint

import (
	"os"
	"testing"
)

func TestDumpRecordingConfig(t *testing.T) {
	path := os.Getenv("DUMP_PZR")
	if path == "" {
		t.Skip("set DUMP_PZR to a .pzr path")
	}
	rd, err := OpenReader(path)
	if err != nil {
		t.Fatal(err)
	}
	out := os.Getenv("DUMP_OUT")
	if out == "" {
		t.Fatal("set DUMP_OUT")
	}
	if err := os.WriteFile(out, rd.Header.Config, 0o644); err != nil {
		t.Fatal(err)
	}
	last := 0
	for _, e := range rd.SnapshotIndex {
		if e.Cycle > last {
			last = e.Cycle
		}
	}
	t.Logf("wrote %d bytes of config to %s", len(rd.Header.Config), out)
	t.Logf("snapshots: %d, last cycle: %d", len(rd.SnapshotIndex), last)
}
