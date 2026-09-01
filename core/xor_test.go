package core

import (
	"errors"
	"testing"
)

func TestFusedBitFromSection2(t *testing.T) {
	sources := []uint8{1, 0, 1}
	fusion := XOR(sources...)
	if got := Recover(fusion, sources[0], sources[2]); got != sources[1] {
		t.Fatalf("recovered %d, want %d", got, sources[1])
	}
	if got := Replace(fusion, sources[1], uint8(1)); got != 1 {
		t.Fatalf("set update = %d, want 1", got)
	}
}

func TestXORSlicesZeroPadsUnequalArrays(t *testing.T) {
	got := XORSlices([]uint64{1, 2, 3}, []uint64{4})
	want := []uint64{5, 2, 3}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("slot %d = %d, want %d", i, got[i], want[i])
		}
	}
}

type demoFusion uint8

func (demoFusion) Recover(int, map[int]uint8) (uint8, error) { return 0, errors.New("demo") }

var _ Fusible[uint8] = demoFusion(0)
