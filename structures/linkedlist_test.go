package structures

import (
	"math/rand"
	"reflect"
	"testing"
)

func TestLinkedListFigure4ValuesAndBitfields(t *testing.T) {
	f, _ := NewFusedLinkedList[uint64](2)
	for _, value := range []uint64{1, 2, 3} {
		_ = f.Insert(0, len(f.Source(0)), value)
	}
	for _, value := range []uint64{8, 16} {
		_ = f.Insert(1, len(f.Source(1)), value)
	}
	if got, want := f.Source(0), []uint64{1, 2, 3}; !reflect.DeepEqual(got, want) {
		t.Fatalf("x1 = %v, want %v", got, want)
	}
	if got, want := f.Source(1), []uint64{8, 16}; !reflect.DeepEqual(got, want) {
		t.Fatalf("x2 = %v, want %v", got, want)
	}
	for _, node := range f.Snapshot() {
		if !anyBit(node.BitField) {
			t.Fatal("Figure 4 fusion contains an empty node")
		}
	}
	got, err := f.Recover(1, map[int][]uint64{0: f.Source(0)})
	if err != nil || !reflect.DeepEqual(got, []uint64{8, 16}) {
		t.Fatalf("recover = %v, %v", got, err)
	}
}

func TestLinkedListRandomArbitraryOperationsRecovery(t *testing.T) {
	f, _ := NewFusedLinkedList[uint64](4)
	rng := rand.New(rand.NewSource(7))
	for range 500 {
		p := rng.Intn(4)
		values := f.Source(p)
		if len(values) == 0 || rng.Intn(3) != 0 {
			_ = f.Insert(p, rng.Intn(len(values)+1), rng.Uint64())
		} else {
			_, _ = f.Delete(p, rng.Intn(len(values)))
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

func TestSetAndPriorityQueueUseLinkedListFusion(t *testing.T) {
	set, _ := NewFusedSet[uint64](2)
	_ = set.Add(0, 3)
	_ = set.Add(0, 3)
	if got := set.Source(0); !reflect.DeepEqual(got, []uint64{3}) {
		t.Fatalf("set = %v", got)
	}
	pq, _ := NewFusedPriorityQueue[uint64](2)
	for _, value := range []uint64{5, 1, 3} {
		_ = pq.Push(0, value)
	}
	if got := pq.Source(0); !reflect.DeepEqual(got, []uint64{1, 3, 5}) {
		t.Fatalf("priority queue = %v", got)
	}
}
