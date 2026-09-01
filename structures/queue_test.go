package structures

import (
	"math/rand"
	"reflect"
	"testing"
)

func TestCircularQueueLemma3ClearsAndWraps(t *testing.T) {
	f, _ := NewFusedQueue[uint64](2, 3)
	for _, value := range []uint64{1, 2, 3} {
		if err := f.Enqueue(0, value); err != nil {
			t.Fatal(err)
		}
	}
	if value, _ := f.Dequeue(0); value != 1 {
		t.Fatalf("dequeue = %d, want 1", value)
	}
	if err := f.Enqueue(0, 4); err != nil {
		t.Fatal(err)
	}
	if err := f.Enqueue(1, 8); err != nil {
		t.Fatal(err)
	}
	if got, want := f.Fusion(), []uint64{4 ^ 8, 2, 3}; !reflect.DeepEqual(got, want) {
		t.Fatalf("fusion %v, want %v", got, want)
	}
	got, err := f.Recover(0, map[int][]uint64{1: f.Source(1)})
	if err != nil || !reflect.DeepEqual(got, []uint64{2, 3, 4}) {
		t.Fatalf("recover = %v, %v", got, err)
	}
}

func TestCircularQueueRandomRecovery(t *testing.T) {
	f, _ := NewFusedQueue[uint64](4, 20)
	rng := rand.New(rand.NewSource(3))
	for range 1000 {
		p := rng.Intn(4)
		size := len(f.Source(p))
		if size == 0 || size < 20 && rng.Intn(3) != 0 {
			if err := f.Enqueue(p, rng.Uint64()); err != nil {
				t.Fatal(err)
			}
		} else if _, err := f.Dequeue(p); err != nil {
			t.Fatal(err)
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
