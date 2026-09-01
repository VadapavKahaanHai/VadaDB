package storage

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestSnapshotAndCheckpointRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "snapshot.bin")
	want := Snapshot{Sequence: 7, Records: []SnapshotRecord{{9, 90}, {1, 10}}}
	if err := WriteSnapshot(path, want); err != nil {
		t.Fatal(err)
	}
	got, err := LoadSnapshot(path)
	want.Records = []SnapshotRecord{{1, 10}, {9, 90}}
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("load = %v, %v; want %v", got, err, want)
	}
	checkpoint := filepath.Join(dir, "snapshot.current")
	if err := WriteCheckpoint(checkpoint, 7); err != nil {
		t.Fatal(err)
	}
	sequence, exists, err := LoadCheckpoint(checkpoint)
	if err != nil || !exists || sequence != 7 {
		t.Fatalf("checkpoint = %d, %v, %v", sequence, exists, err)
	}
}

func TestSnapshotRejectsTruncation(t *testing.T) {
	path := filepath.Join(t.TempDir(), "snapshot.bin")
	if err := os.WriteFile(path, []byte("VDS1"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadSnapshot(path); err == nil {
		t.Fatal("truncated snapshot loaded")
	}
}
