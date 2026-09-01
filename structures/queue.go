package structures

import (
	"fmt"

	"vadadb/core"
)

type FusedQueue[T core.Word] struct {
	sources [][]T
	fusion  []T
	heads   []int
	tails   []int
	counts  []int
}

func NewFusedQueue[T core.Word](sourceCount, capacity int) (*FusedQueue[T], error) {
	if sourceCount < 2 {
		return nil, fmt.Errorf("fusion needs at least two sources")
	}
	if capacity < 1 {
		return nil, fmt.Errorf("capacity must be positive")
	}
	sources := make([][]T, sourceCount)
	for i := range sources {
		sources[i] = make([]T, capacity)
	}
	return &FusedQueue[T]{
		sources: sources,
		fusion:  make([]T, capacity),
		heads:   make([]int, sourceCount),
		tails:   make([]int, sourceCount),
		counts:  make([]int, sourceCount),
	}, nil
}

func (f *FusedQueue[T]) Enqueue(source int, value T) error {
	if err := f.validSource(source); err != nil {
		return err
	}
	if f.counts[source] == len(f.fusion) {
		return fmt.Errorf("source %d queue is full", source)
	}
	index := f.tails[source]
	f.sources[source][index] = value
	f.fusion[index] ^= value
	f.tails[source] = (index + 1) % len(f.fusion)
	f.counts[source]++
	return nil
}

func (f *FusedQueue[T]) Dequeue(source int) (T, error) {
	var zero T
	if err := f.validSource(source); err != nil {
		return zero, err
	}
	if f.counts[source] == 0 {
		return zero, fmt.Errorf("source %d queue is empty", source)
	}
	index := f.heads[source]
	value := f.sources[source][index]
	f.sources[source][index] = zero
	f.fusion[index] ^= value
	f.heads[source] = (index + 1) % len(f.fusion)
	f.counts[source]--
	return value, nil
}

func (f *FusedQueue[T]) Recover(missing int, survivors map[int][]T) ([]T, error) {
	if err := f.validSource(missing); err != nil {
		return nil, err
	}
	if len(survivors) != len(f.sources)-1 {
		return nil, fmt.Errorf("need %d survivors, got %d", len(f.sources)-1, len(survivors))
	}
	physical := append([]T(nil), f.fusion...)
	for source, values := range survivors {
		if source < 0 || source >= len(f.sources) || source == missing {
			return nil, fmt.Errorf("invalid survivor %d", source)
		}
		if len(values) != f.counts[source] {
			return nil, fmt.Errorf("survivor %d has %d values, want %d", source, len(values), f.counts[source])
		}
		for i, value := range values {
			physical[(f.heads[source]+i)%len(physical)] ^= value
		}
	}
	out := make([]T, f.counts[missing])
	for i := range out {
		out[i] = physical[(f.heads[missing]+i)%len(physical)]
	}
	return out, nil
}

func (f *FusedQueue[T]) Fusion() []T { return append([]T(nil), f.fusion...) }

func (f *FusedQueue[T]) Source(source int) []T {
	if f.validSource(source) != nil {
		return nil
	}
	out := make([]T, f.counts[source])
	for i := range out {
		out[i] = f.sources[source][(f.heads[source]+i)%len(f.fusion)]
	}
	return out
}

func (f *FusedQueue[T]) validSource(source int) error {
	if source < 0 || source >= len(f.sources) {
		return fmt.Errorf("source %d out of range", source)
	}
	return nil
}
