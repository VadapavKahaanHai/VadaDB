package structures

import (
	"math/rand"
	"reflect"
	"testing"
)

func TestHashTableLemma5ArrayOfFusedSets(t *testing.T) {
	f, _ := NewFusedHashTable(3, 4)
	_ = f.Put(0, 1, 10)
	_ = f.Put(0, 5, 50) // collision with key 1
	_ = f.Put(1, 2, 20)
	_ = f.Put(2, 1, 99)
	survivors := map[int]map[uint32]uint32{1: f.Source(1), 2: f.Source(2)}
	got, err := f.Recover(0, survivors)
	if err != nil || !reflect.DeepEqual(got, map[uint32]uint32{1: 10, 5: 50}) {
		t.Fatalf("recover = %v, %v", got, err)
	}
}

func TestHashTableRandomUpdatesRecovery(t *testing.T) {
	f, _ := NewFusedHashTable(4, 7)
	rng := rand.New(rand.NewSource(8))
	for range 1000 {
		p := rng.Intn(4)
		key := uint32(rng.Intn(100))
		if rng.Intn(4) == 0 {
			_, _ = f.Delete(p, key)
		} else {
			_ = f.Put(p, key, rng.Uint32())
		}
	}
	for missing := range 4 {
		survivors := map[int]map[uint32]uint32{}
		for i := range 4 {
			if i != missing {
				survivors[i] = f.Source(i)
			}
		}
		got, err := f.Recover(missing, survivors)
		if err != nil || !reflect.DeepEqual(got, f.Source(missing)) {
			t.Fatalf("source %d: got %v, %v; want %v", missing, got, err, f.Source(missing))
		}
	}
}
