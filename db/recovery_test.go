package db

import (
	"errors"
	"math/rand"
	"reflect"
	"testing"
)

func TestCrashClearsShardAndBlocksDependentOperations(t *testing.T) {
	database, _ := New(3)
	_ = database.Put(1, 10)
	_ = database.Put(2, 20)
	if err := database.CrashShard(1); err != nil {
		t.Fatal(err)
	}
	if database.shards[1].data != nil {
		t.Fatal("crashed shard retained its source map")
	}
	if _, _, err := database.Get(1); !errors.Is(err, ErrShardUnavailable) {
		t.Fatalf("get error = %v", err)
	}
	if err := database.Put(4, 40); !errors.Is(err, ErrShardUnavailable) {
		t.Fatalf("put error = %v", err)
	}
	if err := database.Delete(1); !errors.Is(err, ErrShardUnavailable) {
		t.Fatalf("delete error = %v", err)
	}
	if _, err := database.Scan(); !errors.Is(err, ErrShardUnavailable) {
		t.Fatalf("scan error = %v", err)
	}
	if err := database.CrashShard(2); !errors.Is(err, ErrFailureActive) {
		t.Fatalf("second crash error = %v", err)
	}
}

func TestRecoverEveryShardFromFusionAcrossSeeds(t *testing.T) {
	for seed := int64(1); seed <= 4; seed++ {
		for missing := range 3 {
			database, _ := New(3)
			rng := rand.New(rand.NewSource(seed))
			for range 500 {
				key := uint64(rng.Intn(120))
				if rng.Intn(4) == 0 {
					_ = database.Delete(key)
				} else {
					_ = database.Put(key, rng.Uint64())
				}
			}
			want, _ := database.Scan()
			if err := database.CrashShard(missing); err != nil {
				t.Fatal(err)
			}
			if err := database.RecoverShard(missing); err != nil {
				t.Fatalf("seed %d shard %d: %v", seed, missing, err)
			}
			got, err := database.Scan()
			if err != nil || !reflect.DeepEqual(got, want) {
				t.Fatalf("seed %d shard %d: scan %v, %v; want %v", seed, missing, got, err, want)
			}
			key := uint64(missing + 300)
			for database.Route(key) != missing {
				key++
			}
			if err := database.Put(key, 999); err != nil {
				t.Fatalf("post-recovery put: %v", err)
			}
			if value, ok, err := database.Get(key); err != nil || !ok || value != 999 {
				t.Fatalf("post-recovery get = %d, %v, %v", value, ok, err)
			}
		}
	}
}
