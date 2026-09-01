package structures

import (
	"fmt"

	"vadadb/core"
)

type listStackNode[T core.Word] struct {
	value    T
	previous *listStackNode[T]
	next     *listStackNode[T]
}

type FusedListStack[T core.Word] struct {
	sources [][]T
	head    *listStackNode[T]
	tails   []*listStackNode[T]
}

func NewFusedListStack[T core.Word](sourceCount int) (*FusedListStack[T], error) {
	if sourceCount < 2 {
		return nil, fmt.Errorf("fusion needs at least two sources")
	}
	return &FusedListStack[T]{sources: make([][]T, sourceCount), tails: make([]*listStackNode[T], sourceCount)}, nil
}

func (f *FusedListStack[T]) Push(source int, value T) error {
	if err := f.validSource(source); err != nil {
		return err
	}
	node := f.head
	if f.tails[source] != nil {
		node = f.tails[source].next
	}
	if node == nil {
		node = &listStackNode[T]{previous: f.last()}
		if node.previous == nil {
			f.head = node
		} else {
			node.previous.next = node
		}
	}
	node.value ^= value
	f.tails[source] = node
	f.sources[source] = append(f.sources[source], value)
	return nil
}

func (f *FusedListStack[T]) Pop(source int) (T, error) {
	var zero T
	if err := f.validSource(source); err != nil {
		return zero, err
	}
	oldTail := f.tails[source]
	if oldTail == nil {
		return zero, fmt.Errorf("source %d stack is empty", source)
	}
	values := f.sources[source]
	value := values[len(values)-1]
	f.sources[source] = values[:len(values)-1]
	oldTail.value ^= value
	f.tails[source] = oldTail.previous
	if oldTail.next == nil && !f.tailReferenced(oldTail) {
		if oldTail.previous == nil {
			f.head = nil
		} else {
			oldTail.previous.next = nil
		}
	}
	return value, nil
}

func (f *FusedListStack[T]) Recover(missing int, survivors map[int][]T) ([]T, error) {
	if err := f.validSource(missing); err != nil {
		return nil, err
	}
	if len(survivors) != len(f.sources)-1 {
		return nil, fmt.Errorf("need %d survivors, got %d", len(f.sources)-1, len(survivors))
	}
	out := make([]T, len(f.sources[missing]))
	node := f.head
	for i := range out {
		out[i] = node.value
		node = node.next
	}
	for source, values := range survivors {
		if source < 0 || source >= len(f.sources) || source == missing {
			return nil, fmt.Errorf("invalid survivor %d", source)
		}
		for i := 0; i < len(values) && i < len(out); i++ {
			out[i] ^= values[i]
		}
	}
	return out, nil
}

func (f *FusedListStack[T]) Fusion() []T {
	var out []T
	for node := f.head; node != nil; node = node.next {
		out = append(out, node.value)
	}
	return out
}

func (f *FusedListStack[T]) Source(source int) []T {
	if f.validSource(source) != nil {
		return nil
	}
	return append([]T(nil), f.sources[source]...)
}

func (f *FusedListStack[T]) NodeCount() int {
	count := 0
	for node := f.head; node != nil; node = node.next {
		count++
	}
	return count
}

func (f *FusedListStack[T]) last() *listStackNode[T] {
	node := f.head
	if node == nil {
		return nil
	}
	for node.next != nil {
		node = node.next
	}
	return node
}

func (f *FusedListStack[T]) tailReferenced(target *listStackNode[T]) bool {
	for _, tail := range f.tails {
		if tail == target {
			return true
		}
	}
	return false
}

func (f *FusedListStack[T]) validSource(source int) error {
	if source < 0 || source >= len(f.sources) {
		return fmt.Errorf("source %d out of range", source)
	}
	return nil
}
