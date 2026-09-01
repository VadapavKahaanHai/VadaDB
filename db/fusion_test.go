package db

import (
	"math/rand"
	"slices"
	"testing"

	"vadadb/storage"
	"vadadb/structures"
)

func TestFusionInvariantAfterEveryRandomMutation(t *testing.T) {
	database, _ := New(3)
	rng := rand.New(rand.NewSource(31))
	for operation := range 300 {
		key := uint64(rng.Intn(80))
		if rng.Intn(4) == 0 {
			if err := database.Delete(key); err != nil {
				t.Fatal(err)
			}
		} else if err := database.Put(key, rng.Uint64()); err != nil {
			t.Fatal(err)
		}
		for missing := range database.shards {
			got, err := database.fusion.Recover(missing, database.survivors(missing))
			want := shardRecords(database.shards[missing])
			if err != nil || !slices.Equal(got, want) {
				t.Fatalf("operation %d shard %d: recovered %v, %v; want %v", operation, missing, got, err, want)
			}
		}
	}
}

func TestOrphanSourceWALRecordIsNotCommitted(t *testing.T) {
	dir := t.TempDir()
	database, err := Open(dir, 3)
	if err != nil {
		t.Fatal(err)
	}
	orphan := storage.WALRecord{Op: storage.Put, Sequence: database.next, Shard: 1, Key: 1, Value: 99}
	if err := database.shards[1].wal.Append(orphan); err != nil {
		t.Fatal(err)
	}
	if err := database.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := Open(dir, 3)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	if _, ok, _ := reopened.Get(1); ok {
		t.Fatal("source-only WAL tail was treated as committed")
	}
}

func (d *Database) survivors(missing int) map[int][]structures.KVRecord {
	survivors := map[int][]structures.KVRecord{}
	for id, shard := range d.shards {
		if id != missing {
			survivors[id] = shardRecords(shard)
		}
	}
	return survivors
}

func shardRecords(shard *Shard) []structures.KVRecord {
	records := make([]structures.KVRecord, 0, len(shard.data))
	for key, value := range shard.data {
		records = append(records, structures.KVRecord{Key: key, Value: value})
	}
	// Recover returns globally sorted keys.
	for i := 1; i < len(records); i++ {
		for j := i; j > 0 && records[j].Key < records[j-1].Key; j-- {
			records[j], records[j-1] = records[j-1], records[j]
		}
	}
	return records
}
