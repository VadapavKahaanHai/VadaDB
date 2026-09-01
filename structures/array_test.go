package structures

import (
	"math/rand"
	"reflect"
	"testing"
)

func TestFusedArrayLemma1UnequalSizes(t *testing.T) {
	sources := [][]uint64{{1, 0, 1}, {1}, {0, 1}}
	f, err := NewFusedArray(sources...)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := f.Fusion(), []uint64{0, 1, 1}; !reflect.DeepEqual(got, want) {
		t.Fatalf("fusion %v, want %v", got, want)
	}
	survivors := map[int][]uint64{0: sources[0], 2: sources[2]}
	got, err := f.Recover(1, survivors)
	if err != nil || !reflect.DeepEqual(got, sources[1]) {
		t.Fatalf("recover = %v, %v; want %v", got, err, sources[1])
	}
}

func TestFusedArrayRandomUpdatesAndRecovery(t *testing.T) {
	rng := rand.New(rand.NewSource(1))
	sources := [][]uint64{make([]uint64, 8), make([]uint64, 5), make([]uint64, 9)}
	f, _ := NewFusedArray(sources...)
	for range 200 {
		p := rng.Intn(len(sources))
		i := rng.Intn(len(sources[p]))
		value := rng.Uint64()
		sources[p][i] = value
		if err := f.Set(p, i, value); err != nil {
			t.Fatal(err)
		}
	}
	for missing := range sources {
		survivors := map[int][]uint64{}
		for i, source := range sources {
			if i != missing {
				survivors[i] = source
			}
		}
		got, err := f.Recover(missing, survivors)
		if err != nil || !reflect.DeepEqual(got, sources[missing]) {
			t.Fatalf("source %d: got %v, %v; want %v", missing, got, err, sources[missing])
		}
	}
}
