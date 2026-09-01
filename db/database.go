package db

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"

	"vadadb/storage"
	"vadadb/structures"
)

const defaultBucketCount = 16

var (
	ErrShardUnavailable = errors.New("shard unavailable")
	ErrFailureActive    = errors.New("another shard is already failed")
)

type Record struct {
	Key   uint64 `json:"key"`
	Value uint64 `json:"value"`
}

type ShardInfo struct {
	ID      int      `json:"id"`
	Failed  bool     `json:"failed"`
	Records []Record `json:"records"`
}

type StorageMetrics struct {
	SourceRecords          int     `json:"source_records"`
	FusionNodes            int     `json:"fusion_nodes"`
	SourceStorageBytes     uint64  `json:"source_storage_bytes"`
	ReplicationBackupBytes uint64  `json:"replication_backup_bytes"`
	FusionBackupBytes      uint64  `json:"fusion_backup_bytes"`
	SavingPercent          float64 `json:"saving_percent"`
	Assumptions            string  `json:"assumptions"`
}

type DB interface {
	Put(key, value uint64) error
	Get(key uint64) (uint64, bool, error)
	Delete(key uint64) error
	Scan() ([]Record, error)
}

type Shard struct {
	ID     int
	data   map[uint64]uint64
	wal    *storage.WAL
	failed bool
}

type Database struct {
	mu          sync.RWMutex
	shards      []*Shard
	dir         string
	next        uint64
	committed   uint64
	checkpoint  uint64
	hasSnapshot bool
	fusion      *structures.FusedKVHashTable
	fusionWAL   *storage.WAL
}

func New(shardCount int) (*Database, error) {
	if shardCount < 2 {
		return nil, fmt.Errorf("shard count must be at least two")
	}
	fusion, err := structures.NewFusedKVHashTable(shardCount, defaultBucketCount)
	if err != nil {
		return nil, err
	}
	database := &Database{shards: make([]*Shard, shardCount), next: 1, fusion: fusion}
	for id := range database.shards {
		database.shards[id] = &Shard{ID: id, data: map[uint64]uint64{}}
	}
	return database, nil
}

func Open(dir string, shardCount int) (*Database, error) {
	database, err := New(shardCount)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, err
	}
	database.dir = dir
	checkpoint, hasSnapshot, err := storage.LoadCheckpoint(filepath.Join(dir, "snapshot.current"))
	if err != nil {
		return nil, err
	}
	database.checkpoint = checkpoint
	database.hasSnapshot = hasSnapshot
	if hasSnapshot {
		database.committed = checkpoint
		if checkpoint >= database.next {
			database.next = checkpoint + 1
		}
		for id, shard := range database.shards {
			snapshot, err := storage.LoadSnapshot(database.snapshotPath(checkpoint, id))
			if err != nil {
				return nil, err
			}
			if snapshot.Sequence != checkpoint {
				return nil, fmt.Errorf("shard %d snapshot checkpoint mismatch", id)
			}
			for _, record := range snapshot.Records {
				if database.Route(record.Key) != id {
					return nil, fmt.Errorf("snapshot key %d routed to wrong shard", record.Key)
				}
				shard.data[record.Key] = record.Value
			}
		}
	}
	sourceRecords := make([][]storage.WALRecord, shardCount)
	for id, shard := range database.shards {
		shard.wal, err = storage.OpenWAL(filepath.Join(dir, fmt.Sprintf("shard-%d.wal", id)))
		if err != nil {
			database.Close()
			return nil, err
		}
		records, replayErr := shard.wal.Replay()
		if replayErr != nil {
			database.Close()
			return nil, replayErr
		}
		for _, record := range records {
			if int(record.Shard) != id || database.Route(record.Key) != id {
				database.Close()
				return nil, fmt.Errorf("WAL record routed to wrong shard")
			}
			if record.Sequence >= database.next {
				database.next = record.Sequence + 1
			}
		}
		sourceRecords[id] = records
	}
	database.fusionWAL, err = storage.OpenWAL(filepath.Join(dir, "fusion.wal"))
	if err != nil {
		database.Close()
		return nil, err
	}
	commits, err := database.fusionWAL.Replay()
	if err != nil {
		database.Close()
		return nil, err
	}
	committed := make(map[uint64]storage.WALRecord, len(commits))
	for _, record := range commits {
		if _, duplicate := committed[record.Sequence]; duplicate {
			database.Close()
			return nil, fmt.Errorf("duplicate fusion sequence %d", record.Sequence)
		}
		committed[record.Sequence] = record
		if record.Sequence >= database.next {
			database.next = record.Sequence + 1
		}
	}
	seen := make(map[uint64]bool, len(commits))
	for _, records := range sourceRecords {
		for _, record := range records {
			if record.Sequence <= checkpoint {
				continue
			}
			commit, ok := committed[record.Sequence]
			if !ok {
				continue
			}
			if commit != record {
				database.Close()
				return nil, fmt.Errorf("source/fusion WAL mismatch at sequence %d", record.Sequence)
			}
			database.apply(record)
			seen[record.Sequence] = true
			if record.Sequence > database.committed {
				database.committed = record.Sequence
			}
		}
	}
	for sequence := range committed {
		if sequence > checkpoint && !seen[sequence] {
			database.Close()
			return nil, fmt.Errorf("fusion commit %d has no source record", sequence)
		}
	}
	if err := database.rebuildFusion(); err != nil {
		database.Close()
		return nil, err
	}
	return database, nil
}

func (d *Database) Route(key uint64) int { return int(key % uint64(len(d.shards))) }

func (d *Database) Put(key, value uint64) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	shard := d.shards[d.Route(key)]
	if shard.failed {
		return ErrShardUnavailable
	}
	oldValue, exists := shard.data[key]
	index := d.bucketIndex(shard, key)
	record := storage.WALRecord{Op: storage.Put, Sequence: d.next, Shard: uint32(shard.ID), Key: key, Value: value}
	d.next++
	if err := d.commitWAL(shard, record); err != nil {
		return err
	}
	if exists {
		if err := d.fusion.Replace(shard.ID, index, key, oldValue, value); err != nil {
			return err
		}
	} else if err := d.fusion.Insert(shard.ID, index, key, value); err != nil {
		return err
	}
	shard.data[key] = value
	d.committed = record.Sequence
	return nil
}

func (d *Database) Get(key uint64) (uint64, bool, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	shard := d.shards[d.Route(key)]
	if shard.failed {
		return 0, false, ErrShardUnavailable
	}
	value, ok := shard.data[key]
	return value, ok, nil
}

func (d *Database) Delete(key uint64) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	shard := d.shards[d.Route(key)]
	if shard.failed {
		return ErrShardUnavailable
	}
	value, exists := shard.data[key]
	if !exists {
		return nil
	}
	index := d.bucketIndex(shard, key)
	record := storage.WALRecord{Op: storage.Delete, Sequence: d.next, Shard: uint32(shard.ID), Key: key}
	d.next++
	if err := d.commitWAL(shard, record); err != nil {
		return err
	}
	if err := d.fusion.Delete(shard.ID, index, key, value); err != nil {
		return err
	}
	delete(shard.data, key)
	d.committed = record.Sequence
	return nil
}

func (d *Database) Scan() ([]Record, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	var records []Record
	for _, shard := range d.shards {
		if shard.failed {
			return nil, ErrShardUnavailable
		}
		for key, value := range shard.data {
			records = append(records, Record{Key: key, Value: value})
		}
	}
	sort.Slice(records, func(i, j int) bool { return records[i].Key < records[j].Key })
	return records, nil
}

func (d *Database) Shards() []ShardInfo {
	d.mu.RLock()
	defer d.mu.RUnlock()
	shards := make([]ShardInfo, len(d.shards))
	for id, shard := range d.shards {
		shards[id] = ShardInfo{ID: id, Failed: shard.failed}
		if !shard.failed {
			for key, value := range shard.data {
				shards[id].Records = append(shards[id].Records, Record{Key: key, Value: value})
			}
			sort.Slice(shards[id].Records, func(i, j int) bool {
				return shards[id].Records[i].Key < shards[id].Records[j].Key
			})
		}
	}
	return shards
}

func (d *Database) StorageMetrics() StorageMetrics {
	d.mu.RLock()
	defer d.mu.RUnlock()
	records := d.fusion.SourceRecordCount()
	replication := uint64(records * 16)
	fusion := d.fusion.LogicalBytes()
	saving := 0.0
	if replication > 0 {
		saving = (float64(replication) - float64(fusion)) / float64(replication) * 100
	}
	return StorageMetrics{
		SourceRecords:          records,
		FusionNodes:            d.fusion.NodeCount(),
		SourceStorageBytes:     replication,
		ReplicationBackupBytes: replication,
		FusionBackupBytes:      fusion,
		SavingPercent:          saving,
		Assumptions:            "logical payload: 16 bytes per key/value record; fusion node is 16 bytes plus one packed source bit per shard; allocator, map, pointer, bucket, WAL, and snapshot overhead excluded from both backup alternatives",
	}
}

func (d *Database) Snapshot() error {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.dir == "" {
		return fmt.Errorf("in-memory database has no snapshot directory")
	}
	for _, shard := range d.shards {
		if shard.failed {
			return ErrShardUnavailable
		}
	}
	if d.hasSnapshot && d.checkpoint == d.committed {
		return nil
	}
	for id, shard := range d.shards {
		path := d.snapshotPath(d.committed, id)
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return err
		}
		snapshot := storage.Snapshot{Sequence: d.committed, Records: storage.CopySnapshotRecords(shard.data)}
		if err := storage.WriteSnapshot(path, snapshot); err != nil {
			return err
		}
	}
	if err := storage.WriteCheckpoint(filepath.Join(d.dir, "snapshot.current"), d.committed); err != nil {
		return err
	}
	d.checkpoint = d.committed
	d.hasSnapshot = true
	return nil
}

func (d *Database) CrashShard(id int) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	if id < 0 || id >= len(d.shards) {
		return fmt.Errorf("shard %d out of range", id)
	}
	for _, shard := range d.shards {
		if shard.failed && shard.ID != id {
			return ErrFailureActive
		}
	}
	if d.shards[id].failed {
		return ErrShardUnavailable
	}
	d.shards[id].data = nil
	d.shards[id].failed = true
	return nil
}

func (d *Database) RecoverShard(id int) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	if id < 0 || id >= len(d.shards) {
		return fmt.Errorf("shard %d out of range", id)
	}
	if !d.shards[id].failed {
		return fmt.Errorf("shard %d is not failed", id)
	}
	survivors := map[int][]structures.KVRecord{}
	for _, shard := range d.shards {
		if shard.ID == id {
			continue
		}
		if shard.failed {
			return ErrFailureActive
		}
		survivors[shard.ID] = recordsFromMap(shard.data)
	}
	recovered, err := d.fusion.Recover(id, survivors)
	if err != nil {
		return err
	}
	data := make(map[uint64]uint64, len(recovered))
	for _, record := range recovered {
		if d.Route(record.Key) != id {
			return fmt.Errorf("recovered key %d routes to shard %d", record.Key, d.Route(record.Key))
		}
		data[record.Key] = record.Value
	}
	d.shards[id].data = data
	d.shards[id].failed = false
	return nil
}

func (d *Database) Close() error {
	d.mu.Lock()
	defer d.mu.Unlock()
	var first error
	for _, shard := range d.shards {
		if shard.wal != nil {
			if err := shard.wal.Close(); err != nil && first == nil {
				first = err
			}
			shard.wal = nil
		}
	}
	if d.fusionWAL != nil {
		if err := d.fusionWAL.Close(); err != nil && first == nil {
			first = err
		}
		d.fusionWAL = nil
	}
	return first
}

func (d *Database) apply(record storage.WALRecord) {
	shard := d.shards[record.Shard]
	if record.Op == storage.Put {
		shard.data[record.Key] = record.Value
	} else {
		delete(shard.data, record.Key)
	}
}

func (d *Database) commitWAL(shard *Shard, record storage.WALRecord) error {
	if shard.wal == nil {
		return nil
	}
	if err := shard.wal.Append(record); err != nil {
		return err
	}
	if err := d.fusionWAL.Append(record); err != nil {
		return err
	}
	return nil
}

func (d *Database) bucketIndex(shard *Shard, key uint64) int {
	bucket := d.fusion.Bucket(key)
	keys := make([]uint64, 0)
	for current := range shard.data {
		if d.fusion.Bucket(current) == bucket {
			keys = append(keys, current)
		}
	}
	sort.Slice(keys, func(i, j int) bool { return keys[i] < keys[j] })
	return sort.Search(len(keys), func(i int) bool { return keys[i] >= key })
}

func (d *Database) rebuildFusion() error {
	fusion, err := structures.NewFusedKVHashTable(len(d.shards), defaultBucketCount)
	if err != nil {
		return err
	}
	d.fusion = fusion
	for _, shard := range d.shards {
		records := make([]Record, 0, len(shard.data))
		for key, value := range shard.data {
			records = append(records, Record{key, value})
		}
		sort.Slice(records, func(i, j int) bool {
			left, right := fusion.Bucket(records[i].Key), fusion.Bucket(records[j].Key)
			return left < right || left == right && records[i].Key < records[j].Key
		})
		indices := make([]int, defaultBucketCount)
		for _, record := range records {
			bucket := fusion.Bucket(record.Key)
			if err := fusion.Insert(shard.ID, indices[bucket], record.Key, record.Value); err != nil {
				return err
			}
			indices[bucket]++
		}
	}
	return nil
}

func recordsFromMap(data map[uint64]uint64) []structures.KVRecord {
	records := make([]structures.KVRecord, 0, len(data))
	for key, value := range data {
		records = append(records, structures.KVRecord{Key: key, Value: value})
	}
	sort.Slice(records, func(i, j int) bool { return records[i].Key < records[j].Key })
	return records
}

func (d *Database) snapshotPath(sequence uint64, shard int) string {
	return filepath.Join(d.dir, fmt.Sprintf("snapshot-%020d-shard-%d.bin", sequence, shard))
}
