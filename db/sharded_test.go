package db

import (
	"fmt"
	"math/rand"
	"path/filepath"
	"reflect"
	"testing"

	"vadadb/storage"
)

func TestRandomShardedWorkloadSurvivesRestart(t *testing.T) {
	dir := t.TempDir()
	database, err := Open(dir, 5)
	if err != nil {
		t.Fatal(err)
	}
	want := map[uint64]uint64{}
	rng := rand.New(rand.NewSource(21))
	for range 1000 {
		key := uint64(rng.Intn(200))
		if rng.Intn(4) == 0 {
			if err := database.Delete(key); err != nil {
				t.Fatal(err)
			}
			delete(want, key)
		} else {
			value := rng.Uint64()
			if err := database.Put(key, value); err != nil {
				t.Fatal(err)
			}
			want[key] = value
		}
	}
	if err := database.Close(); err != nil {
		t.Fatal(err)
	}

	for id := range 5 {
		wal, err := storage.OpenWAL(filepath.Join(dir, fmt.Sprintf("shard-%d.wal", id)))
		if err != nil {
			t.Fatal(err)
		}
		records, err := wal.Replay()
		wal.Close()
		if err != nil {
			t.Fatal(err)
		}
		for _, record := range records {
			if int(record.Shard) != id || int(record.Key%5) != id {
				t.Fatalf("record %+v found in shard %d WAL", record, id)
			}
		}
	}

	reopened, err := Open(dir, 5)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	got, err := reopened.Scan()
	if err != nil {
		t.Fatal(err)
	}
	wantRecords := make([]Record, 0, len(want))
	for key, value := range want {
		wantRecords = append(wantRecords, Record{key, value})
	}
	// Scan is sorted; route the expected records through a temporary DB to reuse
	// the public ordering contract rather than duplicate its sort here.
	memory, _ := New(5)
	for _, record := range wantRecords {
		_ = memory.Put(record.Key, record.Value)
	}
	wantRecords, _ = memory.Scan()
	if !reflect.DeepEqual(got, wantRecords) {
		t.Fatalf("restart scan mismatch: got %v want %v", got, wantRecords)
	}
}
