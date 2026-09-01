package structures

import (
	"math/rand"
	"reflect"
	"testing"
)

func TestArrayStackFigure1(t *testing.T) {
	f, _ := NewFusedStack[uint64](2)
	for _, op := range []struct {
		p int
		v uint64
	}{{0, 1}, {0, 2}, {1, 4}, {1, 8}, {1, 16}} {
		if err := f.Push(op.p, op.v); err != nil {
			t.Fatal(err)
		}
	}
	if got, want := f.Fusion(), []uint64{1 ^ 4, 2 ^ 8, 16}; !reflect.DeepEqual(got, want) {
		t.Fatalf("fusion %v, want %v", got, want)
	}
	got, err := f.Recover(0, map[int][]uint64{1: f.Source(1)})
	if err != nil || !reflect.DeepEqual(got, []uint64{1, 2}) {
		t.Fatalf("recover = %v, %v", got, err)
	}
}

func TestArrayStackRandomOperationsRecoverEverySource(t *testing.T) {
	f, _ := NewFusedStack[uint64](4)
	rng := rand.New(rand.NewSource(2))
	for range 500 {
		p := rng.Intn(4)
		if len(f.Source(p)) == 0 || rng.Intn(3) != 0 {
			if err := f.Push(p, rng.Uint64()); err != nil {
				t.Fatal(err)
			}
		} else if _, err := f.Pop(p); err != nil {
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
