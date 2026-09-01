package db

import (
	"reflect"
	"testing"
)

func TestSnapshotThenReplayLaterWAL(t *testing.T) {
	dir := t.TempDir()
	database, err := Open(dir, 3)
	if err != nil {
		t.Fatal(err)
	}
	_ = database.Put(1, 10)
	_ = database.Put(2, 20)
	_ = database.Put(3, 30)
	if err := database.Snapshot(); err != nil {
		t.Fatal(err)
	}
	if err := database.Snapshot(); err != nil {
		t.Fatalf("unchanged snapshot should be idempotent: %v", err)
	}
	_ = database.Put(4, 40)
	_ = database.Put(2, 22)
	_ = database.Delete(1)
	want, _ := database.Scan()
	if err := database.Close(); err != nil {
		t.Fatal(err)
	}

	reopened, err := Open(dir, 3)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	got, err := reopened.Scan()
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("snapshot restart = %v, %v; want %v", got, err, want)
	}
	for missing := range 3 {
		recovered, err := reopened.fusion.Recover(missing, reopened.survivors(missing))
		if err != nil || !reflect.DeepEqual(recovered, shardRecords(reopened.shards[missing])) {
			t.Fatalf("rebuilt fusion shard %d = %v, %v", missing, recovered, err)
		}
	}
}

func TestSnapshotRequiresDurableHealthyDatabase(t *testing.T) {
	memory, _ := New(3)
	if err := memory.Snapshot(); err == nil {
		t.Fatal("in-memory snapshot succeeded")
	}
	database, _ := Open(t.TempDir(), 3)
	defer database.Close()
	_ = database.CrashShard(0)
	if err := database.Snapshot(); err != ErrShardUnavailable {
		t.Fatalf("snapshot error = %v", err)
	}
}
