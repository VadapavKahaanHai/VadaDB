package db

import "testing"

func TestLogicalStorageMetricsAndFailedShard(t *testing.T) {
	database, err := New(3)
	if err != nil {
		t.Fatal(err)
	}
	for key, value := range map[uint64]uint64{0: 10, 48: 20, 16: 30, 32: 40} {
		if err := database.Put(key, value); err != nil {
			t.Fatal(err)
		}
	}
	metrics := database.StorageMetrics()
	if metrics.SourceRecords != 4 || metrics.FusionNodes != 2 || metrics.SourceStorageBytes != 64 || metrics.ReplicationBackupBytes != 64 || metrics.FusionBackupBytes != 34 || metrics.SavingPercent != 46.875 {
		t.Fatalf("metrics = %#v", metrics)
	}
	if err := database.CrashShard(1); err != nil {
		t.Fatal(err)
	}
	if failed := database.StorageMetrics(); failed != metrics {
		t.Fatalf("metrics changed during failure: before %#v, after %#v", metrics, failed)
	}
}
