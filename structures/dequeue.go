package structures

import (
	"container/list"
	"fmt"

	"vadadb/core"
)

type FusedDeque[T core.Word] struct {
	sources []*list.List
	head    *fusedQueueNode[T]
	tail    *fusedQueueNode[T]
	heads   []*fusedQueueNode[T]
	tails   []*fusedQueueNode[T]
}

func NewFusedDeque[T core.Word](sourceCount int) (*FusedDeque[T], error) {
	if sourceCount < 2 {
		return nil, fmt.Errorf("fusion needs at least two sources")
	}
	sources := make([]*list.List, sourceCount)
	for i := range sources {
		sources[i] = list.New()
	}
	return &FusedDeque[T]{sources: sources, heads: make([]*fusedQueueNode[T], sourceCount), tails: make([]*fusedQueueNode[T], sourceCount)}, nil
}

func (f *FusedDeque[T]) PushBack(source int, value T) error {
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
	f.sources[source].PushBack(value)
	return nil
}

func (f *FusedDeque[T]) PushFront(source int, value T) error {
	if err := f.validSource(source); err != nil {
		return err
	}
	var node *fusedQueueNode[T]
	if f.heads[source] == nil {
		node = f.tail
		if node == nil {
			node = f.prependNode()
		}
		f.tails[source] = node
	} else {
		node = f.heads[source].previous
		if node == nil {
			node = f.prependNode()
		}
	}
	node.value ^= value
	node.refCount++
	f.heads[source] = node
	f.sources[source].PushFront(value)
	return nil
}

func (f *FusedDeque[T]) PopFront(source int) (T, error) {
	return f.pop(source, true)
}

func (f *FusedDeque[T]) PopBack(source int) (T, error) {
	return f.pop(source, false)
}

func (f *FusedDeque[T]) pop(source int, front bool) (T, error) {
	var zero T
	if err := f.validSource(source); err != nil {
		return zero, err
	}
	if f.sources[source].Len() == 0 {
		return zero, fmt.Errorf("source %d dequeue is empty", source)
	}
	node := f.tails[source]
	element := f.sources[source].Back()
	if front {
		node = f.heads[source]
		element = f.sources[source].Front()
	}
	value := element.Value.(T)
	f.sources[source].Remove(element)
	node.value ^= value
	node.refCount--
	if f.sources[source].Len() == 0 {
		f.heads[source], f.tails[source] = nil, nil
	} else if front {
		f.heads[source] = node.next
	} else {
		f.tails[source] = node.previous
	}
	if node.refCount == 0 {
		f.removeNode(node)
	}
	if front {
		newHead := f.heads[source]
		if newHead != nil && newHead.previous != nil && newHead.refCount == 1 && newHead.previous.refCount == 1 {
			f.merge(newHead.previous, newHead)
		}
	} else {
		newTail := f.tails[source]
		if newTail != nil && newTail.next != nil && newTail.refCount == 1 && newTail.next.refCount == 1 {
			f.merge(newTail, newTail.next)
		}
	}
	return value, nil
}

func (f *FusedDeque[T]) Recover(missing int, survivors map[int][]T) ([]T, error) {
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
		if len(sourceValues) != f.sources[source].Len() {
			return nil, fmt.Errorf("survivor %d has %d values, want %d", source, len(sourceValues), f.sources[source].Len())
		}
		node := f.heads[source]
		for _, value := range sourceValues {
			values[node] ^= value
			node = node.next
		}
	}
	out := make([]T, 0, f.sources[missing].Len())
	for node := f.heads[missing]; len(out) < f.sources[missing].Len(); node = node.next {
		out = append(out, values[node])
	}
	return out, nil
}

func (f *FusedDeque[T]) Source(source int) []T {
	if f.validSource(source) != nil {
		return nil
	}
	out := make([]T, 0, f.sources[source].Len())
	for element := f.sources[source].Front(); element != nil; element = element.Next() {
		out = append(out, element.Value.(T))
	}
	return out
}

func (f *FusedDeque[T]) NodeCount() int {
	count := 0
	for node := f.head; node != nil; node = node.next {
		count++
	}
	return count
}

func (f *FusedDeque[T]) appendNode() *fusedQueueNode[T] {
	node := &fusedQueueNode[T]{previous: f.tail}
	if f.tail == nil {
		f.head = node
	} else {
		f.tail.next = node
	}
	f.tail = node
	return node
}

func (f *FusedDeque[T]) prependNode() *fusedQueueNode[T] {
	node := &fusedQueueNode[T]{next: f.head}
	if f.head == nil {
		f.tail = node
	} else {
		f.head.previous = node
	}
	f.head = node
	return node
}

func (f *FusedDeque[T]) removeNode(node *fusedQueueNode[T]) {
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

func (f *FusedDeque[T]) merge(left, right *fusedQueueNode[T]) {
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

func (f *FusedDeque[T]) validSource(source int) error {
	if source < 0 || source >= len(f.sources) {
		return fmt.Errorf("source %d out of range", source)
	}
	return nil
}
