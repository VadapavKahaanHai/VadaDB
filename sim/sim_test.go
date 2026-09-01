package sim

import (
	"reflect"
	"testing"
)

func TestReferenceTableMatchesPaperTable1(t *testing.T) {
	got := ReferenceTable()
	wantLast := ReferenceRow{50, 38.7, 43.7, 2.2, 30.5, 2.6}
	if len(got) != 5 || !reflect.DeepEqual(got[4], wantLast) {
		t.Fatalf("table = %v", got)
	}
}

func TestRunIsDeterministicAndFusionUsesFewerNodes(t *testing.T) {
	config := Config{Processes: 10, Operations: 2000, InsertPercent: 60, Seed: 1}
	first, err := Run(config)
	if err != nil {
		t.Fatal(err)
	}
	second, _ := Run(config)
	if !reflect.DeepEqual(first, second) {
		t.Fatal("same seed produced different results")
	}
	for _, result := range first {
		if result.FusionNodes <= 0 || result.FusionNodes >= result.ReplicationNodes || result.Savings <= 1 {
			t.Fatalf("invalid space result: %+v", result)
		}
	}
}
