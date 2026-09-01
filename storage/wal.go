package storage

import (
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"sync"
)

type Operation uint8

const (
	Put Operation = iota + 1
	Delete
)

const recordSize = 40

var magic = [4]byte{'V', 'D', 'B', '1'}

type WALRecord struct {
	Op       Operation
	Sequence uint64
	Shard    uint32
	Key      uint64
	Value    uint64
}

type WAL struct {
	mu   sync.Mutex
	file *os.File
}

func OpenWAL(path string) (*WAL, error) {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR|os.O_APPEND, 0o600)
	if err != nil {
		return nil, err
	}
	return &WAL{file: file}, nil
}

func (w *WAL) Append(record WALRecord) error {
	data, err := encodeRecord(record)
	if err != nil {
		return err
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	written, err := w.file.Write(data)
	if err == nil && written != len(data) {
		err = io.ErrShortWrite
	}
	return err
}

func (w *WAL) Sync() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.file.Sync()
}

func (w *WAL) Replay() ([]WALRecord, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if _, err := w.file.Seek(0, io.SeekStart); err != nil {
		return nil, err
	}
	var records []WALRecord
	buffer := make([]byte, recordSize)
	for {
		_, err := io.ReadFull(w.file, buffer)
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("read WAL record %d: %w", len(records), err)
		}
		record, err := decodeRecord(buffer)
		if err != nil {
			return nil, fmt.Errorf("decode WAL record %d: %w", len(records), err)
		}
		records = append(records, record)
	}
	_, err := w.file.Seek(0, io.SeekEnd)
	return records, err
}

func (w *WAL) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.file.Close()
}

func encodeRecord(record WALRecord) ([]byte, error) {
	if record.Op != Put && record.Op != Delete {
		return nil, fmt.Errorf("invalid WAL operation %d", record.Op)
	}
	data := make([]byte, recordSize)
	copy(data[:4], magic[:])
	data[4] = 1
	data[5] = byte(record.Op)
	binary.BigEndian.PutUint64(data[8:16], record.Sequence)
	binary.BigEndian.PutUint32(data[16:20], record.Shard)
	binary.BigEndian.PutUint64(data[24:32], record.Key)
	binary.BigEndian.PutUint64(data[32:40], record.Value)
	return data, nil
}

func decodeRecord(data []byte) (WALRecord, error) {
	if len(data) != recordSize || string(data[:4]) != string(magic[:]) || data[4] != 1 {
		return WALRecord{}, fmt.Errorf("invalid WAL header")
	}
	record := WALRecord{
		Op:       Operation(data[5]),
		Sequence: binary.BigEndian.Uint64(data[8:16]),
		Shard:    binary.BigEndian.Uint32(data[16:20]),
		Key:      binary.BigEndian.Uint64(data[24:32]),
		Value:    binary.BigEndian.Uint64(data[32:40]),
	}
	if record.Op != Put && record.Op != Delete {
		return WALRecord{}, fmt.Errorf("invalid WAL operation %d", record.Op)
	}
	return record, nil
}
