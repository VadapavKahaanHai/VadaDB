package tests

import (
	"reflect"
	"testing"

	"vadadb/db"
)

func TestV1Lifecycle(t *testing.T) {
	dir := t.TempDir()
	database, err := db.Open(dir, 3)
	if err != nil {
		t.Fatal(err)
	}
	expected := map[uint64]uint64{}
	for key := uint64(0); key < 12; key++ {
		expected[key] = key*100 + 7
		if err := database.Put(key, expected[key]); err != nil {
			t.Fatal(err)
		}
	}
	if err := database.Close(); err != nil {
		t.Fatal(err)
	}
	database, err = db.Open(dir, 3)
	if err != nil {
		t.Fatal(err)
	}
	if err := database.CrashShard(1); err != nil {
		t.Fatal(err)
	}
	if _, _, err := database.Get(1); err != db.ErrShardUnavailable {
		t.Fatalf("failed-shard GET error = %v", err)
	}
	if err := database.RecoverShard(1); err != nil {
		t.Fatal(err)
	}
	expected[13] = 1307
	if err := database.Put(13, expected[13]); err != nil {
		t.Fatal(err)
	}
	delete(expected, 0)
	if err := database.Delete(0); err != nil {
		t.Fatal(err)
	}
	if err := database.Snapshot(); err != nil {
		t.Fatal(err)
	}
	expected[14] = 1407
	if err := database.Put(14, expected[14]); err != nil {
		t.Fatal(err)
	}
	if err := database.Close(); err != nil {
		t.Fatal(err)
	}
	database, err = db.Open(dir, 3)
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	records, err := database.Scan()
	if err != nil {
		t.Fatal(err)
	}
	got := make(map[uint64]uint64, len(records))
	for _, record := range records {
		got[record.Key] = record.Value
	}
	if !reflect.DeepEqual(got, expected) {
		t.Fatalf("restart state = %v, want %v", got, expected)
	}
}
