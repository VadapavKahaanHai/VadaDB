package structures

import (
	"math/rand"
	"reflect"
	"testing"
)

func TestDequeSection43MirroredOperations(t *testing.T) {
	f, _ := NewFusedDeque[uint64](2)
	_ = f.PushBack(0, 2)
	_ = f.PushFront(0, 1)
	_ = f.PushBack(0, 3)
	_ = f.PushFront(1, 8)
	_ = f.PushBack(1, 16)
	if value, _ := f.PopFront(0); value != 1 {
		t.Fatalf("pop front = %d, want 1", value)
	}
	if value, _ := f.PopBack(1); value != 16 {
		t.Fatalf("pop back = %d, want 16", value)
	}
	got, err := f.Recover(0, map[int][]uint64{1: f.Source(1)})
	if err != nil || !reflect.DeepEqual(got, []uint64{2, 3}) {
		t.Fatalf("recover = %v, %v", got, err)
	}
}

func TestDequeRandomRecovery(t *testing.T) {
	f, _ := NewFusedDeque[uint64](4)
	rng := rand.New(rand.NewSource(6))
	for range 1000 {
		p := rng.Intn(4)
		if len(f.Source(p)) == 0 || rng.Intn(3) != 0 {
			if rng.Intn(2) == 0 {
				_ = f.PushFront(p, rng.Uint64())
			} else {
				_ = f.PushBack(p, rng.Uint64())
			}
		} else if rng.Intn(2) == 0 {
			_, _ = f.PopFront(p)
		} else {
			_, _ = f.PopBack(p)
		}
	}
	for missing := range 4 {
		survivors := map[int][]uint64{}
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
