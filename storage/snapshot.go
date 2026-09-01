package storage

import (
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
	"sort"
)

const snapshotHeaderSize = 24

var snapshotMagic = [4]byte{'V', 'D', 'S', '1'}
var checkpointMagic = [4]byte{'V', 'D', 'C', '1'}

type SnapshotRecord struct {
	Key   uint64
	Value uint64
}

type Snapshot struct {
	Sequence uint64
	Records  []SnapshotRecord
}

func WriteSnapshot(path string, snapshot Snapshot) error {
	sort.Slice(snapshot.Records, func(i, j int) bool { return snapshot.Records[i].Key < snapshot.Records[j].Key })
	data := make([]byte, snapshotHeaderSize+len(snapshot.Records)*16)
	copy(data[:4], snapshotMagic[:])
	data[4] = 1
	binary.BigEndian.PutUint64(data[8:16], snapshot.Sequence)
	binary.BigEndian.PutUint64(data[16:24], uint64(len(snapshot.Records)))
	for i, record := range snapshot.Records {
		offset := snapshotHeaderSize + i*16
		binary.BigEndian.PutUint64(data[offset:offset+8], record.Key)
		binary.BigEndian.PutUint64(data[offset+8:offset+16], record.Value)
	}
	return writeFile(path, data, os.O_EXCL)
}

func LoadSnapshot(path string) (Snapshot, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Snapshot{}, err
	}
	if len(data) < snapshotHeaderSize || string(data[:4]) != string(snapshotMagic[:]) || data[4] != 1 {
		return Snapshot{}, fmt.Errorf("invalid snapshot header")
	}
	count := binary.BigEndian.Uint64(data[16:24])
	if count > uint64((len(data)-snapshotHeaderSize)/16) || snapshotHeaderSize+int(count)*16 != len(data) {
		return Snapshot{}, fmt.Errorf("invalid snapshot length")
	}
	snapshot := Snapshot{Sequence: binary.BigEndian.Uint64(data[8:16]), Records: make([]SnapshotRecord, int(count))}
	seen := make(map[uint64]bool, count)
	for i := range snapshot.Records {
		offset := snapshotHeaderSize + i*16
		record := SnapshotRecord{binary.BigEndian.Uint64(data[offset : offset+8]), binary.BigEndian.Uint64(data[offset+8 : offset+16])}
		if seen[record.Key] {
			return Snapshot{}, fmt.Errorf("duplicate snapshot key %d", record.Key)
		}
		seen[record.Key] = true
		snapshot.Records[i] = record
	}
	return snapshot, nil
}

func WriteCheckpoint(path string, sequence uint64) error {
	data := make([]byte, 12)
	copy(data[:4], checkpointMagic[:])
	binary.BigEndian.PutUint64(data[4:], sequence)
	temporary := path + ".tmp"
	if err := writeFile(temporary, data, os.O_TRUNC); err != nil {
		return err
	}
	if err := os.Rename(temporary, path); err == nil {
		return nil
	}
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	return os.Rename(temporary, path)
}

func LoadCheckpoint(path string) (uint64, bool, error) {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, err
	}
	if len(data) != 12 || string(data[:4]) != string(checkpointMagic[:]) {
		return 0, false, fmt.Errorf("invalid snapshot checkpoint")
	}
	return binary.BigEndian.Uint64(data[4:]), true, nil
}

func writeFile(path string, data []byte, mode int) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|mode, 0o600)
	if err != nil {
		return err
	}
	if _, err = file.Write(data); err == nil {
		err = file.Sync()
	}
	if closeErr := file.Close(); err == nil {
		err = closeErr
	}
	return err
}

func CopySnapshotRecords(data map[uint64]uint64) []SnapshotRecord {
	records := make([]SnapshotRecord, 0, len(data))
	for key, value := range data {
		records = append(records, SnapshotRecord{Key: key, Value: value})
	}
	return records
}
