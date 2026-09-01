package main

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"vadadb/db"
)

const helpText = `Commands:
  PUT <key> <value>
  GET <key>
  DELETE <key>
  SCAN
  CRASH SHARD <id>
  RECOVER SHARD <id>
  SNAPSHOT
  SHOW SHARDS
  SHOW STORAGE
  HELP
  EXIT

Commands are case-insensitive. A trailing semicolon is optional.`

type command struct {
	name       string
	key, value uint64
	shard      int
}

func main() { os.Exit(run()) }

func run() int {
	dir := os.Getenv("VADADB_DATA")
	if dir == "" {
		dir = "data"
	}
	database, err := db.Open(dir, 3)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		return 1
	}

	args := os.Args[1:]
	if len(args) > 0 && args[0] == "--" {
		args = args[1:]
	}
	code := 0
	if len(args) > 0 {
		cmd, err := parseOneShot(strings.Join(args, " "))
		if err == nil {
			_, err = executeCommand(cmd, database, os.Stdout)
		}
		if err != nil {
			fmt.Fprintln(os.Stderr, "error:", err)
			code = 1
		}
	} else {
		code = runInteractive(database, dir)
	}
	if err := database.Close(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		return 1
	}
	return code
}

func runInteractive(database *db.Database, dir string) int {
	fmt.Printf("VadaDB CLI (data: %s). Type HELP for commands.\n", dir)
	scanner := bufio.NewScanner(os.Stdin)
	for {
		fmt.Print("vadadb> ")
		if !scanner.Scan() {
			break
		}
		quit, err := runCommand(scanner.Text(), database, os.Stdout)
		if err != nil {
			fmt.Fprintln(os.Stderr, "error:", err)
		}
		if quit {
			return 0
		}
	}
	if err := scanner.Err(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		return 1
	}
	return 0
}

func runCommand(line string, database *db.Database, out io.Writer) (bool, error) {
	cmd, err := parseCommand(line)
	if err != nil {
		return false, err
	}
	return executeCommand(cmd, database, out)
}

func executeCommand(cmd command, database *db.Database, out io.Writer) (bool, error) {
	switch cmd.name {
	case "PUT":
		if err := database.Put(cmd.key, cmd.value); err != nil {
			return false, err
		}
		fmt.Fprintf(out, "OK: %d = %d\n", cmd.key, cmd.value)
	case "GET":
		value, found, err := database.Get(cmd.key)
		if err != nil {
			return false, err
		}
		if !found {
			return false, errors.New("key not found")
		}
		fmt.Fprintln(out, value)
	case "DELETE":
		if err := database.Delete(cmd.key); err != nil {
			return false, err
		}
		fmt.Fprintln(out, "OK")
	case "SCAN":
		records, err := database.Scan()
		if err != nil {
			return false, err
		}
		if len(records) == 0 {
			fmt.Fprintln(out, "(empty)")
		}
		for _, record := range records {
			fmt.Fprintf(out, "%d\t%d\n", record.Key, record.Value)
		}
	case "CRASH":
		if err := database.CrashShard(cmd.shard); err != nil {
			return false, err
		}
		fmt.Fprintf(out, "OK: shard %d crashed\n", cmd.shard)
	case "RECOVER":
		if err := database.RecoverShard(cmd.shard); err != nil {
			return false, err
		}
		fmt.Fprintf(out, "OK: shard %d recovered\n", cmd.shard)
	case "SNAPSHOT":
		if err := database.Snapshot(); err != nil {
			return false, err
		}
		fmt.Fprintln(out, "OK: snapshot created")
	case "SHOW SHARDS":
		for _, shard := range database.Shards() {
			status := "healthy"
			if shard.Failed {
				status = "unavailable"
			}
			fmt.Fprintf(out, "SHARD %d\t%s\t%d records\n", shard.ID, status, len(shard.Records))
			for _, record := range shard.Records {
				fmt.Fprintf(out, "  %d\t%d\n", record.Key, record.Value)
			}
		}
	case "SHOW STORAGE":
		metrics := database.StorageMetrics()
		fmt.Fprintf(out, "Source payload: %d B\nReplication backup: %d B\nFusion backup: %d B\nSaving: %.1f%%\nAssumptions: %s\n",
			metrics.SourceStorageBytes, metrics.ReplicationBackupBytes, metrics.FusionBackupBytes, metrics.SavingPercent, metrics.Assumptions)
	case "HELP":
		fmt.Fprintln(out, helpText)
	case "EXIT":
		return true, nil
	}
	return false, nil
}

func parseCommand(line string) (command, error) {
	line = strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(line), ";"))
	fields := strings.Fields(line)
	if len(fields) == 0 {
		return command{}, errors.New("enter a command or type HELP")
	}
	for i := range fields {
		fields[i] = strings.ToUpper(fields[i])
	}

	switch fields[0] {
	case "PUT":
		if len(fields) != 3 {
			return command{}, errors.New("usage: PUT <key> <value>")
		}
		key, err := parseUint(fields[1], "key")
		if err != nil {
			return command{}, err
		}
		value, err := parseUint(fields[2], "value")
		return command{name: "PUT", key: key, value: value}, err
	case "GET", "DELETE":
		if len(fields) != 2 {
			return command{}, fmt.Errorf("usage: %s <key>", fields[0])
		}
		key, err := parseUint(fields[1], "key")
		return command{name: fields[0], key: key}, err
	case "SCAN", "SNAPSHOT", "HELP":
		if len(fields) == 1 {
			return command{name: fields[0]}, nil
		}
	case "EXIT", "QUIT":
		if len(fields) == 1 {
			return command{name: "EXIT"}, nil
		}
	case "CRASH", "RECOVER":
		if len(fields) != 3 || fields[1] != "SHARD" {
			return command{}, fmt.Errorf("usage: %s SHARD <id>", fields[0])
		}
		id, err := strconv.Atoi(fields[2])
		if err != nil || id < 0 {
			return command{}, errors.New("shard id must be a non-negative integer")
		}
		return command{name: fields[0], shard: id}, nil
	case "SHOW":
		if len(fields) == 2 && (fields[1] == "SHARDS" || fields[1] == "STORAGE") {
			return command{name: "SHOW " + fields[1]}, nil
		}
		return command{}, errors.New("usage: SHOW SHARDS or SHOW STORAGE")
	}
	return command{}, errors.New("unknown command; type HELP")
}

func parseOneShot(line string) (command, error) {
	cmd, err := parseCommand(line)
	if err == nil && (cmd.name == "CRASH" || cmd.name == "RECOVER") {
		err = errors.New("CRASH and RECOVER require an interactive CLI session")
	}
	return cmd, err
}

func parseUint(value, name string) (uint64, error) {
	parsed, err := strconv.ParseUint(value, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("%s must be an unsigned decimal integer", name)
	}
	return parsed, nil
}
