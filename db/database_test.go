package db

import (
	"reflect"
	"testing"
)

func TestDatabaseCRUDScanAndRouting(t *testing.T) {
	database, err := New(3)
	if err != nil {
		t.Fatal(err)
	}
	for key, shard := range map[uint64]int{0: 0, 1: 1, 2: 2, 3: 0, 4: 1} {
		if got := database.Route(key); got != shard {
			t.Fatalf("route(%d) = %d, want %d", key, got, shard)
		}
	}
	if _, ok, err := database.Get(9); err != nil || ok {
		t.Fatalf("missing get = ok %v, err %v", ok, err)
	}
	if err := database.Put(4, 40); err != nil {
		t.Fatal(err)
	}
	if err := database.Put(1, 10); err != nil {
		t.Fatal(err)
	}
	if err := database.Put(4, 44); err != nil {
		t.Fatal(err)
	}
	if value, ok, err := database.Get(4); err != nil || !ok || value != 44 {
		t.Fatalf("get = %d, %v, %v", value, ok, err)
	}
	if got, _ := database.Scan(); !reflect.DeepEqual(got, []Record{{1, 10}, {4, 44}}) {
		t.Fatalf("scan = %v", got)
	}
	if err := database.Delete(4); err != nil {
		t.Fatal(err)
	}
	if err := database.Delete(4); err != nil {
		t.Fatal(err)
	}
	if _, ok, _ := database.Get(4); ok {
		t.Fatal("deleted key still exists")
	}
}

func TestDatabaseRejectsInvalidShardCount(t *testing.T) {
	for _, count := range []int{-1, 0, 1} {
		if _, err := New(count); err == nil {
			t.Fatalf("New(%d) succeeded", count)
		}
	}
}
