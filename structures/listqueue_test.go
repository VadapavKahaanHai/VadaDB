package structures

import (
	"math/rand"
	"reflect"
	"testing"
)

func TestFusedQueueFigure3LiteralWorkedExample(t *testing.T) {
	const a1, a2, a3, a4, b1, b2 uint64 = 1, 2, 3, 4, 8, 16
	f, _ := NewFusedListQueue[uint64](2)
	mustEnqueue := func(p int, value uint64) {
		t.Helper()
		if err := f.Enqueue(p, value); err != nil {
			t.Fatal(err)
		}
	}
	mustDequeue := func(p int) {
		t.Helper()
		if _, err := f.Dequeue(p); err != nil {
			t.Fatal(err)
		}
	}

	mustEnqueue(0, a1)
	mustEnqueue(1, b1)
	assertQueueSnapshot(t, f.Snapshot(), []FusedQueueNode[uint64]{{a1 ^ b1, 2}}, []int{0, 0}, []int{0, 0}) // Figure 3(i)

	mustEnqueue(0, a2)
	assertQueueSnapshot(t, f.Snapshot(), []FusedQueueNode[uint64]{{a1 ^ b1, 2}, {a2, 1}}, []int{0, 0}, []int{1, 0}) // Figure 3(ii)

	mustEnqueue(0, a3)
	mustEnqueue(0, a4)
	mustEnqueue(1, b2)
	mustDequeue(0)
	assertQueueSnapshot(t, f.Snapshot(), []FusedQueueNode[uint64]{{b1, 1}, {a2 ^ b2, 2}, {a3, 1}, {a4, 1}}, []int{1, 0}, []int{3, 1}) // Figure 3(iii)

	mustDequeue(0)
	assertQueueSnapshot(t, f.Snapshot(), []FusedQueueNode[uint64]{{b1, 1}, {b2 ^ a3, 2}, {a4, 1}}, []int{1, 0}, []int{2, 1}) // Figure 3(iv), after merge
}

func TestFusedQueueRandomRecoveryAndSpace(t *testing.T) {
	f, _ := NewFusedListQueue[uint64](4)
	rng := rand.New(rand.NewSource(5))
	for range 1000 {
		p := rng.Intn(4)
		if len(f.Source(p)) == 0 || rng.Intn(3) != 0 {
			if err := f.Enqueue(p, rng.Uint64()); err != nil {
				t.Fatal(err)
			}
		} else if _, err := f.Dequeue(p); err != nil {
			t.Fatal(err)
		}
		total := 0
		for i := range 4 {
			total += len(f.Source(i))
		}
		if total > 1 && f.NodeCount() >= total {
			t.Fatalf("nodes %d must be less than total %d", f.NodeCount(), total)
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

func assertQueueSnapshot(t *testing.T, got ListQueueSnapshot[uint64], nodes []FusedQueueNode[uint64], heads, tails []int) {
	t.Helper()
	if !reflect.DeepEqual(got.Nodes, nodes) || !reflect.DeepEqual(got.Heads, heads) || !reflect.DeepEqual(got.Tails, tails) {
		t.Fatalf("snapshot = %+v, want nodes=%v heads=%v tails=%v", got, nodes, heads, tails)
	}
}
