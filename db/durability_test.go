package db

import (
	"reflect"
	"testing"
)

func TestWALRestartRecovery(t *testing.T) {
	dir := t.TempDir()
	database, err := Open(dir, 3)
	if err != nil {
		t.Fatal(err)
	}
	_ = database.Put(1, 10)
	_ = database.Put(2, 20)
	_ = database.Put(1, 11)
	_ = database.Put(4, 40)
	_ = database.Delete(2)
	_ = database.Delete(99)
	if err := database.Close(); err != nil {
		t.Fatal(err)
	}

	reopened, err := Open(dir, 3)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	got, err := reopened.Scan()
	if err != nil || !reflect.DeepEqual(got, []Record{{1, 11}, {4, 40}}) {
		t.Fatalf("restart scan = %v, %v", got, err)
	}
	if _, ok, _ := reopened.Get(2); ok {
		t.Fatal("deleted record returned after restart")
	}
}
