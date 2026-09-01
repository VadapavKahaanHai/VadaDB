package main

import (
	"bytes"
	"strings"
	"testing"

	"vadadb/db"
)

func TestCLIWorkflow(t *testing.T) {
	database, err := db.Open(t.TempDir(), 3)
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()

	var out bytes.Buffer
	commands := []string{"put 42 9001;", "GET 42", "CRASH SHARD 0", "RECOVER SHARD 0", "SCAN", "SHOW STORAGE"}
	for _, input := range commands {
		if _, err := runCommand(input, database, &out); err != nil {
			t.Fatalf("%s: %v", input, err)
		}
	}
	for _, want := range []string{"OK: 42 = 9001", "9001", "shard 0 crashed", "shard 0 recovered", "42\t9001", "Fusion backup:"} {
		if !strings.Contains(out.String(), want) {
			t.Fatalf("output missing %q: %s", want, out.String())
		}
	}
}

func TestParseCommandRejectsInvalidInput(t *testing.T) {
	for _, input := range []string{"PUT -1 2", "GET", "CRASH 1", "SHOW TABLES"} {
		if _, err := parseCommand(input); err == nil {
			t.Fatalf("parseCommand(%q) succeeded", input)
		}
	}
	if _, err := parseOneShot("CRASH SHARD 0"); err == nil {
		t.Fatal("one-shot crash succeeded")
	}
}
