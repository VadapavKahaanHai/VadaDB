package structures

import (
	"fmt"

	"vadadb/core"
)

type fusedQueueNode[T core.Word] struct {
	value    T
	refCount int
	previous *fusedQueueNode[T]
	next     *fusedQueueNode[T]
}

type FusedQueueNode[T core.Word] struct {
	Value    T
	RefCount int
}

type ListQueueSnapshot[T core.Word] struct {
	Nodes []FusedQueueNode[T]
	Heads []int
	Tails []int
}

type FusedListQueue[T core.Word] struct {
	sources [][]T
	head    *fusedQueueNode[T]
	tail    *fusedQueueNode[T]
	heads   []*fusedQueueNode[T]
	tails   []*fusedQueueNode[T]
}

func NewFusedListQueue[T core.Word](sourceCount int) (*FusedListQueue[T], error) {
	if sourceCount < 2 {
		return nil, fmt.Errorf("fusion needs at least two sources")
	}
	return &FusedListQueue[T]{
		sources: make([][]T, sourceCount),
		heads:   make([]*fusedQueueNode[T], sourceCount),
		tails:   make([]*fusedQueueNode[T], sourceCount),
	}, nil
}

func (f *FusedListQueue[T]) Enqueue(source int, value T) error {
	if err := f.validSource(source); err != nil {
		return err
	}
	var node *fusedQueueNode[T]
	if f.tails[source] == nil {
		node = f.head
		if node == nil {
			node = f.appendNode()
		}
		f.heads[source] = node
	} else {
		node = f.tails[source].next
		if node == nil {
			node = f.appendNode()
		}
	}
	node.value ^= value
	node.refCount++
	f.tails[source] = node
	f.sources[source] = append(f.sources[source], value)
	return nil
}

func (f *FusedListQueue[T]) Dequeue(source int) (T, error) {
	var zero T
	if err := f.validSource(source); err != nil {
		return zero, err
	}
	oldHead := f.heads[source]
	if oldHead == nil {
		return zero, fmt.Errorf("source %d queue is empty", source)
	}
	value := f.sources[source][0]
	f.sources[source] = f.sources[source][1:]
	oldHead.value ^= value
	oldHead.refCount--
	if len(f.sources[source]) == 0 {
		f.heads[source], f.tails[source] = nil, nil
	} else {
		f.heads[source] = oldHead.next
	}
	if oldHead.refCount == 0 {
		f.removeNode(oldHead)
	}
	newHead := f.heads[source]
	if newHead != nil && newHead.previous != nil && newHead.refCount == 1 && newHead.previous.refCount == 1 {
		f.merge(newHead.previous, newHead)
	}
	return value, nil
}

func (f *FusedListQueue[T]) Recover(missing int, survivors map[int][]T) ([]T, error) {
	if err := f.validSource(missing); err != nil {
		return nil, err
	}
	if len(survivors) != len(f.sources)-1 {
		return nil, fmt.Errorf("need %d survivors, got %d", len(f.sources)-1, len(survivors))
	}
	values := map[*fusedQueueNode[T]]T{}
	for node := f.head; node != nil; node = node.next {
		values[node] = node.value
	}
	for source, sourceValues := range survivors {
		if source < 0 || source >= len(f.sources) || source == missing {
			return nil, fmt.Errorf("invalid survivor %d", source)
		}
		node := f.heads[source]
		for _, value := range sourceValues {
			if node == nil {
				return nil, fmt.Errorf("survivor %d is longer than its fused path", source)
			}
			values[node] ^= value
			node = node.next
		}
		if len(sourceValues) != f.sourceLength(source) {
			return nil, fmt.Errorf("survivor %d has %d values, want %d", source, len(sourceValues), f.sourceLength(source))
		}
	}
	length := f.sourceLength(missing)
	out := make([]T, 0, length)
	for node := f.heads[missing]; len(out) < length; node = node.next {
		out = append(out, values[node])
	}
	return out, nil
}

func (f *FusedListQueue[T]) Source(source int) []T {
	if f.validSource(source) != nil {
		return nil
	}
	return append([]T(nil), f.sources[source]...)
}

func (f *FusedListQueue[T]) NodeCount() int {
	count := 0
	for node := f.head; node != nil; node = node.next {
		count++
	}
	return count
}

func (f *FusedListQueue[T]) Snapshot() ListQueueSnapshot[T] {
	indices := map[*fusedQueueNode[T]]int{}
	snapshot := ListQueueSnapshot[T]{Heads: make([]int, len(f.heads)), Tails: make([]int, len(f.tails))}
	for node := f.head; node != nil; node = node.next {
		indices[node] = len(snapshot.Nodes)
		snapshot.Nodes = append(snapshot.Nodes, FusedQueueNode[T]{Value: node.value, RefCount: node.refCount})
	}
	for i := range snapshot.Heads {
		snapshot.Heads[i], snapshot.Tails[i] = -1, -1
		if f.heads[i] != nil {
			snapshot.Heads[i] = indices[f.heads[i]]
		}
		if f.tails[i] != nil {
			snapshot.Tails[i] = indices[f.tails[i]]
		}
	}
	return snapshot
}

func (f *FusedListQueue[T]) appendNode() *fusedQueueNode[T] {
	node := &fusedQueueNode[T]{previous: f.tail}
	if f.tail == nil {
		f.head = node
	} else {
		f.tail.next = node
	}
	f.tail = node
	return node
}

func (f *FusedListQueue[T]) removeNode(node *fusedQueueNode[T]) {
	if node.previous == nil {
		f.head = node.next
	} else {
		node.previous.next = node.next
	}
	if node.next == nil {
		f.tail = node.previous
	} else {
		node.next.previous = node.previous
	}
}

func (f *FusedListQueue[T]) merge(left, right *fusedQueueNode[T]) {
	left.value ^= right.value
	left.refCount += right.refCount
	for i := range f.heads {
		if f.heads[i] == right {
			f.heads[i] = left
		}
		if f.tails[i] == right {
			f.tails[i] = left
		}
	}
	f.removeNode(right)
}

func (f *FusedListQueue[T]) sourceLength(source int) int {
	if f.heads[source] == nil {
		return 0
	}
	length := 1
	for node := f.heads[source]; node != f.tails[source]; node = node.next {
		length++
	}
	return length
}

func (f *FusedListQueue[T]) validSource(source int) error {
	if source < 0 || source >= len(f.sources) {
		return fmt.Errorf("source %d out of range", source)
	}
	return nil
}
