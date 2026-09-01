package storage

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestWALAppendSyncReplay(t *testing.T) {
	path := filepath.Join(t.TempDir(), "shard.wal")
	w, err := OpenWAL(path)
	if err != nil {
		t.Fatal(err)
	}
	want := []WALRecord{{Put, 1, 2, 7, 70}, {Delete, 2, 2, 7, 0}}
	for _, record := range want {
		if err := w.Append(record); err != nil {
			t.Fatal(err)
		}
	}
	got, err := w.Replay()
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("replay = %v, %v", got, err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestWALRejectsTruncatedRecord(t *testing.T) {
	path := filepath.Join(t.TempDir(), "shard.wal")
	if err := os.WriteFile(path, []byte("VDB1"), 0o600); err != nil {
		t.Fatal(err)
	}
	w, err := OpenWAL(path)
	if err != nil {
		t.Fatal(err)
	}
	defer w.Close()
	if _, err := w.Replay(); err == nil {
		t.Fatal("truncated WAL replay succeeded")
	}
}
