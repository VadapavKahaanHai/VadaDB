package structures

import (
	"math/rand"
	"reflect"
	"testing"
)

func TestListStackSection41TailSharing(t *testing.T) {
	f, _ := NewFusedListStack[uint64](2)
	for _, op := range []struct {
		p int
		v uint64
	}{{0, 1}, {1, 8}, {0, 2}, {1, 16}, {0, 4}} {
		if err := f.Push(op.p, op.v); err != nil {
			t.Fatal(err)
		}
	}
	if got, want := f.Fusion(), []uint64{1 ^ 8, 2 ^ 16, 4}; !reflect.DeepEqual(got, want) {
		t.Fatalf("fusion %v, want %v", got, want)
	}
	if _, err := f.Pop(0); err != nil {
		t.Fatal(err)
	}
	if f.NodeCount() != 2 {
		t.Fatalf("nodes = %d, want 2", f.NodeCount())
	}
}

func TestListStackRandomRecovery(t *testing.T) {
	f, _ := NewFusedListStack[uint64](3)
	rng := rand.New(rand.NewSource(4))
	for range 500 {
		p := rng.Intn(3)
		if len(f.Source(p)) == 0 || rng.Intn(3) != 0 {
			if err := f.Push(p, rng.Uint64()); err != nil {
				t.Fatal(err)
			}
		} else if _, err := f.Pop(p); err != nil {
			t.Fatal(err)
		}
	}
	for missing := range 3 {
		survivors := map[int][]uint64{}
		for i := range 3 {
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
