package structures

import (
	"fmt"
	"sort"
)

type KVRecord struct {
	Key   uint64 `json:"key"`
	Value uint64 `json:"value"`
}

type FusedKVNode struct {
	Key      uint64 `json:"key_xor"`
	Value    uint64 `json:"value_xor"`
	BitField []bool `json:"bit_field"`
}

type kvNode struct {
	key, value uint64
	bits       []bool
	previous   *kvNode
	next       *kvNode
}

type kvBucket struct {
	head, tail *kvNode
}

// FusedKVHashTable implements Section 4.5 as a fixed array of the fused sets
// from Section 4.4. It stores occupancy metadata but no source records.
type FusedKVHashTable struct {
	sourceCount int
	buckets     []kvBucket
	sizes       [][]int
}

func NewFusedKVHashTable(sourceCount, bucketCount int) (*FusedKVHashTable, error) {
	if sourceCount < 2 {
		return nil, fmt.Errorf("fusion needs at least two sources")
	}
	if bucketCount < 1 {
		return nil, fmt.Errorf("bucket count must be positive")
	}
	sizes := make([][]int, sourceCount)
	for source := range sizes {
		sizes[source] = make([]int, bucketCount)
	}
	return &FusedKVHashTable{sourceCount: sourceCount, buckets: make([]kvBucket, bucketCount), sizes: sizes}, nil
}

func (f *FusedKVHashTable) Bucket(key uint64) int {
	return int(key % uint64(len(f.buckets)))
}

func (f *FusedKVHashTable) Insert(source, index int, key, value uint64) error {
	bucketIndex := f.Bucket(key)
	if err := f.validPosition(source, bucketIndex, index, true); err != nil {
		return err
	}
	bucket := &f.buckets[bucketIndex]
	var previous, nextSource *kvNode
	seen := 0
	for node := bucket.head; node != nil; node = node.next {
		if node.bits[source] {
			if seen == index {
				nextSource = node
				break
			}
			previous = node
			seen++
		}
	}
	candidate := bucket.head
	if previous != nil {
		candidate = previous.next
	}
	if candidate == nextSource || candidate == nil {
		candidate = f.insertBefore(bucket, nextSource)
	}
	candidate.key ^= key
	candidate.value ^= value
	candidate.bits[source] = true
	f.sizes[source][bucketIndex]++
	return nil
}

func (f *FusedKVHashTable) Replace(source, index int, key, oldValue, newValue uint64) error {
	bucketIndex := f.Bucket(key)
	if err := f.validPosition(source, bucketIndex, index, false); err != nil {
		return err
	}
	node := f.nodeAt(bucketIndex, source, index)
	node.value ^= oldValue ^ newValue
	return nil
}

func (f *FusedKVHashTable) Delete(source, index int, key, value uint64) error {
	bucketIndex := f.Bucket(key)
	if err := f.validPosition(source, bucketIndex, index, false); err != nil {
		return err
	}
	node := f.nodeAt(bucketIndex, source, index)
	node.key ^= key
	node.value ^= value
	node.bits[source] = false
	f.sizes[source][bucketIndex]--
	f.mergeNodes(&f.buckets[bucketIndex], node)
	return nil
}

func (f *FusedKVHashTable) Recover(missing int, survivors map[int][]KVRecord) ([]KVRecord, error) {
	if missing < 0 || missing >= f.sourceCount {
		return nil, fmt.Errorf("source %d out of range", missing)
	}
	if len(survivors) != f.sourceCount-1 {
		return nil, fmt.Errorf("need %d survivors, got %d", f.sourceCount-1, len(survivors))
	}
	grouped := make(map[int][][]KVRecord, len(survivors))
	for source, records := range survivors {
		if source < 0 || source >= f.sourceCount || source == missing {
			return nil, fmt.Errorf("invalid survivor %d", source)
		}
		grouped[source] = make([][]KVRecord, len(f.buckets))
		for _, record := range records {
			bucket := f.Bucket(record.Key)
			grouped[source][bucket] = append(grouped[source][bucket], record)
		}
		for bucket := range f.buckets {
			sort.Slice(grouped[source][bucket], func(i, j int) bool {
				return grouped[source][bucket][i].Key < grouped[source][bucket][j].Key
			})
			if len(grouped[source][bucket]) != f.sizes[source][bucket] {
				return nil, fmt.Errorf("survivor %d bucket %d has %d records, want %d", source, bucket, len(grouped[source][bucket]), f.sizes[source][bucket])
			}
		}
	}
	var recovered []KVRecord
	for bucketIndex := range f.buckets {
		values := map[*kvNode]KVRecord{}
		for node := f.buckets[bucketIndex].head; node != nil; node = node.next {
			values[node] = KVRecord{Key: node.key, Value: node.value}
		}
		for source := range survivors {
			index := 0
			for node := f.buckets[bucketIndex].head; node != nil; node = node.next {
				if node.bits[source] {
					record := grouped[source][bucketIndex][index]
					value := values[node]
					value.Key ^= record.Key
					value.Value ^= record.Value
					values[node] = value
					index++
				}
			}
		}
		for node := f.buckets[bucketIndex].head; node != nil; node = node.next {
			if node.bits[missing] {
				recovered = append(recovered, values[node])
			}
		}
	}
	sort.Slice(recovered, func(i, j int) bool { return recovered[i].Key < recovered[j].Key })
	return recovered, nil
}

func (f *FusedKVHashTable) Snapshot() [][]FusedKVNode {
	out := make([][]FusedKVNode, len(f.buckets))
	for bucketIndex := range f.buckets {
		for node := f.buckets[bucketIndex].head; node != nil; node = node.next {
			out[bucketIndex] = append(out[bucketIndex], FusedKVNode{Key: node.key, Value: node.value, BitField: append([]bool(nil), node.bits...)})
		}
	}
	return out
}

func (f *FusedKVHashTable) NodeCount() int {
	total := 0
	for _, bucket := range f.Snapshot() {
		total += len(bucket)
	}
	return total
}

func (f *FusedKVHashTable) BucketCount() int { return len(f.buckets) }

func (f *FusedKVHashTable) SourceRecordCount() int {
	total := 0
	for source := range f.sizes {
		for _, size := range f.sizes[source] {
			total += size
		}
	}
	return total
}

func (f *FusedKVHashTable) LogicalBytes() uint64 {
	bitFieldBytes := (f.sourceCount + 7) / 8
	return uint64(f.NodeCount() * (16 + bitFieldBytes))
}

func (f *FusedKVHashTable) validPosition(source, bucket, index int, insert bool) error {
	if source < 0 || source >= f.sourceCount {
		return fmt.Errorf("source %d out of range", source)
	}
	limit := f.sizes[source][bucket]
	if index < 0 || index > limit || !insert && index == limit {
		return fmt.Errorf("index %d out of range", index)
	}
	return nil
}

func (f *FusedKVHashTable) nodeAt(bucketIndex, source, index int) *kvNode {
	for node := f.buckets[bucketIndex].head; node != nil; node = node.next {
		if node.bits[source] {
			if index == 0 {
				return node
			}
			index--
		}
	}
	return nil
}

func (f *FusedKVHashTable) insertBefore(bucket *kvBucket, next *kvNode) *kvNode {
	node := &kvNode{bits: make([]bool, f.sourceCount), next: next}
	if next == nil {
		node.previous = bucket.tail
		if bucket.tail == nil {
			bucket.head = node
		} else {
			bucket.tail.next = node
		}
		bucket.tail = node
		return node
	}
	node.previous = next.previous
	next.previous = node
	if node.previous == nil {
		bucket.head = node
	} else {
		node.previous.next = node
	}
	return node
}

func (f *FusedKVHashTable) mergeNodes(bucket *kvBucket, node *kvNode) {
	if !anyBit(node.bits) {
		previous, next := node.previous, node.next
		f.removeNode(bucket, node)
		if previous != nil {
			node = previous
		} else {
			node = next
		}
	}
	if node == nil {
		return
	}
	for node.previous != nil && disjoint(node.previous.bits, node.bits) {
		node = f.merge(bucket, node.previous, node)
	}
	for node.next != nil && disjoint(node.bits, node.next.bits) {
		f.merge(bucket, node, node.next)
	}
}

func (f *FusedKVHashTable) merge(bucket *kvBucket, left, right *kvNode) *kvNode {
	left.key ^= right.key
	left.value ^= right.value
	for source := range left.bits {
		left.bits[source] = left.bits[source] || right.bits[source]
	}
	f.removeNode(bucket, right)
	return left
}

func (f *FusedKVHashTable) removeNode(bucket *kvBucket, node *kvNode) {
	if node.previous == nil {
		bucket.head = node.next
	} else {
		node.previous.next = node.next
	}
	if node.next == nil {
		bucket.tail = node.previous
	} else {
		node.next.previous = node.previous
	}
}
