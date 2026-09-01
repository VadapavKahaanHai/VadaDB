package structures

import (
	"fmt"
	"sort"

	"vadadb/core"
)

type fusedListNode[T core.Word] struct {
	value    T
	bits     []bool
	previous *fusedListNode[T]
	next     *fusedListNode[T]
}

type FusedListNode[T core.Word] struct {
	Value    T
	BitField []bool
}

type FusedLinkedList[T core.Word] struct {
	sources [][]T
	head    *fusedListNode[T]
	tail    *fusedListNode[T]
}

func NewFusedLinkedList[T core.Word](sourceCount int) (*FusedLinkedList[T], error) {
	if sourceCount < 2 {
		return nil, fmt.Errorf("fusion needs at least two sources")
	}
	return &FusedLinkedList[T]{sources: make([][]T, sourceCount)}, nil
}

func (f *FusedLinkedList[T]) Insert(source, index int, value T) error {
	if err := f.validSource(source); err != nil {
		return err
	}
	if index < 0 || index > len(f.sources[source]) {
		return fmt.Errorf("index %d out of range", index)
	}
	var previous, nextSource *fusedListNode[T]
	seen := 0
	for node := f.head; node != nil; node = node.next {
		if node.bits[source] {
			if seen == index {
				nextSource = node
				break
			}
			previous = node
			seen++
		}
	}
	candidate := f.head
	if previous != nil {
		candidate = previous.next
	}
	for candidate != nextSource && candidate != nil && candidate.bits[source] {
		candidate = candidate.next
	}
	if candidate == nextSource || candidate == nil {
		candidate = f.insertBefore(nextSource)
	}
	candidate.value ^= value
	candidate.bits[source] = true
	values := f.sources[source]
	values = append(values, 0)
	copy(values[index+1:], values[index:])
	values[index] = value
	f.sources[source] = values
	return nil
}

func (f *FusedLinkedList[T]) Delete(source, index int) (T, error) {
	var zero T
	if err := f.validSource(source); err != nil {
		return zero, err
	}
	if index < 0 || index >= len(f.sources[source]) {
		return zero, fmt.Errorf("index %d out of range", index)
	}
	node := f.nodeAt(source, index)
	value := f.sources[source][index]
	copy(f.sources[source][index:], f.sources[source][index+1:])
	f.sources[source] = f.sources[source][:len(f.sources[source])-1]
	node.value ^= value
	node.bits[source] = false
	f.mergeNodes(node)
	return value, nil
}

func (f *FusedLinkedList[T]) Recover(missing int, survivors map[int][]T) ([]T, error) {
	if err := f.validSource(missing); err != nil {
		return nil, err
	}
	if len(survivors) != len(f.sources)-1 {
		return nil, fmt.Errorf("need %d survivors, got %d", len(f.sources)-1, len(survivors))
	}
	values := map[*fusedListNode[T]]T{}
	for node := f.head; node != nil; node = node.next {
		values[node] = node.value
	}
	for source, sourceValues := range survivors {
		if source < 0 || source >= len(f.sources) || source == missing {
			return nil, fmt.Errorf("invalid survivor %d", source)
		}
		index := 0
		for node := f.head; node != nil; node = node.next {
			if node.bits[source] {
				if index >= len(sourceValues) {
					return nil, fmt.Errorf("survivor %d is shorter than its bitfield", source)
				}
				values[node] ^= sourceValues[index]
				index++
			}
		}
		if index != len(sourceValues) {
			return nil, fmt.Errorf("survivor %d has %d values, want %d", source, len(sourceValues), index)
		}
	}
	var out []T
	for node := f.head; node != nil; node = node.next {
		if node.bits[missing] {
			out = append(out, values[node])
		}
	}
	return out, nil
}

func (f *FusedLinkedList[T]) Source(source int) []T {
	if f.validSource(source) != nil {
		return nil
	}
	return append([]T(nil), f.sources[source]...)
}

func (f *FusedLinkedList[T]) Snapshot() []FusedListNode[T] {
	var out []FusedListNode[T]
	for node := f.head; node != nil; node = node.next {
		out = append(out, FusedListNode[T]{Value: node.value, BitField: append([]bool(nil), node.bits...)})
	}
	return out
}

func (f *FusedLinkedList[T]) NodeCount() int { return len(f.Snapshot()) }

func (f *FusedLinkedList[T]) nodeAt(source, index int) *fusedListNode[T] {
	for node := f.head; node != nil; node = node.next {
		if node.bits[source] {
			if index == 0 {
				return node
			}
			index--
		}
	}
	return nil
}

func (f *FusedLinkedList[T]) insertBefore(next *fusedListNode[T]) *fusedListNode[T] {
	node := &fusedListNode[T]{bits: make([]bool, len(f.sources)), next: next}
	if next == nil {
		node.previous = f.tail
		if f.tail == nil {
			f.head = node
		} else {
			f.tail.next = node
		}
		f.tail = node
		return node
	}
	node.previous = next.previous
	next.previous = node
	if node.previous == nil {
		f.head = node
	} else {
		node.previous.next = node
	}
	return node
}

func (f *FusedLinkedList[T]) mergeNodes(node *fusedListNode[T]) {
	if !anyBit(node.bits) {
		previous, next := node.previous, node.next
		f.removeNode(node)
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
		node = f.merge(node.previous, node)
	}
	for node.next != nil && disjoint(node.bits, node.next.bits) {
		f.merge(node, node.next)
	}
}

func (f *FusedLinkedList[T]) merge(left, right *fusedListNode[T]) *fusedListNode[T] {
	left.value ^= right.value
	for i := range left.bits {
		left.bits[i] = left.bits[i] || right.bits[i]
	}
	f.removeNode(right)
	return left
}

func (f *FusedLinkedList[T]) removeNode(node *fusedListNode[T]) {
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

func (f *FusedLinkedList[T]) validSource(source int) error {
	if source < 0 || source >= len(f.sources) {
		return fmt.Errorf("source %d out of range", source)
	}
	return nil
}

func anyBit(bits []bool) bool {
	for _, bit := range bits {
		if bit {
			return true
		}
	}
	return false
}

func disjoint(a, b []bool) bool {
	for i := range a {
		if a[i] && b[i] {
			return false
		}
	}
	return true
}

type FusedSet[T core.Word] struct{ *FusedLinkedList[T] }

func NewFusedSet[T core.Word](sourceCount int) (*FusedSet[T], error) {
	list, err := NewFusedLinkedList[T](sourceCount)
	if err != nil {
		return nil, err
	}
	return &FusedSet[T]{list}, nil
}

func (f *FusedSet[T]) Add(source int, value T) error {
	if err := f.validSource(source); err != nil {
		return err
	}
	if f.Contains(source, value) {
		return nil
	}
	return f.Insert(source, len(f.sources[source]), value)
}

func (f *FusedSet[T]) Remove(source int, value T) (bool, error) {
	for i, current := range f.Source(source) {
		if current == value {
			_, err := f.Delete(source, i)
			return true, err
		}
	}
	return false, f.validSource(source)
}

func (f *FusedSet[T]) Contains(source int, value T) bool {
	for _, current := range f.Source(source) {
		if current == value {
			return true
		}
	}
	return false
}

type FusedPriorityQueue[T core.Word] struct{ *FusedLinkedList[T] }

func NewFusedPriorityQueue[T core.Word](sourceCount int) (*FusedPriorityQueue[T], error) {
	list, err := NewFusedLinkedList[T](sourceCount)
	if err != nil {
		return nil, err
	}
	return &FusedPriorityQueue[T]{list}, nil
}

func (f *FusedPriorityQueue[T]) Push(source int, value T) error {
	values := f.Source(source)
	index := sort.Search(len(values), func(i int) bool { return values[i] >= value })
	return f.Insert(source, index, value)
}

func (f *FusedPriorityQueue[T]) PopMin(source int) (T, error) {
	return f.Delete(source, 0)
}
