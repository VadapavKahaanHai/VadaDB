package structures

import (
	"math"
	"reflect"
	"testing"
)

func TestFusedKVHashTableLemma5Recovery(t *testing.T) {
	fusion, err := NewFusedKVHashTable(3, 4)
	if err != nil {
		t.Fatal(err)
	}
	// Keys 1, 5, and 9 collide in the same fixed set bucket (Section 4.5).
	_ = fusion.Insert(0, 0, 1, 10)
	_ = fusion.Insert(0, 1, 5, 50)
	_ = fusion.Insert(1, 0, 9, 90)
	_ = fusion.Insert(2, 0, math.MaxUint64, math.MaxUint64)
	_ = fusion.Replace(0, 1, 5, 50, 55)
	_ = fusion.Delete(1, 0, 9, 90)
	survivors := map[int][]KVRecord{
		1: {},
		2: {{math.MaxUint64, math.MaxUint64}},
	}
	got, err := fusion.Recover(0, survivors)
	want := []KVRecord{{1, 10}, {5, 55}}
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("recover = %v, %v; want %v", got, err, want)
	}
}

func TestFusedKVHashTableZeroIsData(t *testing.T) {
	fusion, _ := NewFusedKVHashTable(2, 2)
	_ = fusion.Insert(0, 0, 0, 0)
	_ = fusion.Insert(1, 0, 2, 0)
	got, err := fusion.Recover(0, map[int][]KVRecord{1: {{2, 0}}})
	if err != nil || !reflect.DeepEqual(got, []KVRecord{{0, 0}}) {
		t.Fatalf("recover zero = %v, %v", got, err)
	}
}
