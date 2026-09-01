package structures

import (
	"fmt"
	"sort"
)

type FusedHashTable struct {
	sources []map[uint32]uint32
	buckets []*FusedLinkedList[uint64]
}

func NewFusedHashTable(sourceCount, bucketCount int) (*FusedHashTable, error) {
	if sourceCount < 2 {
		return nil, fmt.Errorf("fusion needs at least two sources")
	}
	if bucketCount < 1 {
		return nil, fmt.Errorf("bucket count must be positive")
	}
	f := &FusedHashTable{sources: make([]map[uint32]uint32, sourceCount), buckets: make([]*FusedLinkedList[uint64], bucketCount)}
	for i := range f.sources {
		f.sources[i] = map[uint32]uint32{}
	}
	for i := range f.buckets {
		f.buckets[i], _ = NewFusedLinkedList[uint64](sourceCount)
	}
	return f, nil
}

func (f *FusedHashTable) Put(source int, key, value uint32) error {
	if err := f.validSource(source); err != nil {
		return err
	}
	if old, ok := f.sources[source][key]; ok {
		if old == value {
			return nil
		}
		f.deletePacked(source, key, old)
	}
	word := packEntry(key, value)
	bucket := f.buckets[f.bucket(key)]
	values := bucket.Source(source)
	index := sort.Search(len(values), func(i int) bool { return values[i] >= word })
	if err := bucket.Insert(source, index, word); err != nil {
		return err
	}
	f.sources[source][key] = value
	return nil
}

func (f *FusedHashTable) Delete(source int, key uint32) (bool, error) {
	if err := f.validSource(source); err != nil {
		return false, err
	}
	value, ok := f.sources[source][key]
	if !ok {
		return false, nil
	}
	f.deletePacked(source, key, value)
	delete(f.sources[source], key)
	return true, nil
}

func (f *FusedHashTable) Get(source int, key uint32) (uint32, bool) {
	if f.validSource(source) != nil {
		return 0, false
	}
	value, ok := f.sources[source][key]
	return value, ok
}

func (f *FusedHashTable) Recover(missing int, survivors map[int]map[uint32]uint32) (map[uint32]uint32, error) {
	if err := f.validSource(missing); err != nil {
		return nil, err
	}
	if len(survivors) != len(f.sources)-1 {
		return nil, fmt.Errorf("need %d survivors, got %d", len(f.sources)-1, len(survivors))
	}
	out := map[uint32]uint32{}
	for bucketIndex, bucket := range f.buckets {
		bucketSurvivors := map[int][]uint64{}
		for source, entries := range survivors {
			if source < 0 || source >= len(f.sources) || source == missing {
				return nil, fmt.Errorf("invalid survivor %d", source)
			}
			words := []uint64{}
			for key, value := range entries {
				if f.bucket(key) == bucketIndex {
					words = append(words, packEntry(key, value))
				}
			}
			sort.Slice(words, func(i, j int) bool { return words[i] < words[j] })
			bucketSurvivors[source] = words
		}
		words, err := bucket.Recover(missing, bucketSurvivors)
		if err != nil {
			return nil, fmt.Errorf("bucket %d: %w", bucketIndex, err)
		}
		for _, word := range words {
			key, value := unpackEntry(word)
			out[key] = value
		}
	}
	return out, nil
}

func (f *FusedHashTable) Source(source int) map[uint32]uint32 {
	if f.validSource(source) != nil {
		return nil
	}
	out := make(map[uint32]uint32, len(f.sources[source]))
	for key, value := range f.sources[source] {
		out[key] = value
	}
	return out
}

func (f *FusedHashTable) NodeCount() int {
	total := 0
	for _, bucket := range f.buckets {
		total += bucket.NodeCount()
	}
	return total
}

func (f *FusedHashTable) deletePacked(source int, key, value uint32) {
	word := packEntry(key, value)
	bucket := f.buckets[f.bucket(key)]
	values := bucket.Source(source)
	index := sort.Search(len(values), func(i int) bool { return values[i] >= word })
	_, _ = bucket.Delete(source, index)
}

func (f *FusedHashTable) bucket(key uint32) int { return int(key % uint32(len(f.buckets))) }

func (f *FusedHashTable) validSource(source int) error {
	if source < 0 || source >= len(f.sources) {
		return fmt.Errorf("source %d out of range", source)
	}
	return nil
}

func packEntry(key, value uint32) uint64 { return uint64(key)<<32 | uint64(value) }

func unpackEntry(word uint64) (uint32, uint32) { return uint32(word >> 32), uint32(word) }
