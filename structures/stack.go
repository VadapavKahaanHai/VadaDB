package structures

import (
	"fmt"

	"vadadb/core"
)

type FusedStack[T core.Word] struct {
	sources [][]T
	fusion  []T
	tops    []int
}

func NewFusedStack[T core.Word](sourceCount int) (*FusedStack[T], error) {
	if sourceCount < 2 {
		return nil, fmt.Errorf("fusion needs at least two sources")
	}
	return &FusedStack[T]{sources: make([][]T, sourceCount), tops: make([]int, sourceCount)}, nil
}

func (f *FusedStack[T]) Push(source int, value T) error {
	if err := f.validSource(source); err != nil {
		return err
	}
	depth := f.tops[source]
	if depth == len(f.fusion) {
		f.fusion = append(f.fusion, 0)
	}
	f.fusion[depth] ^= value
	f.sources[source] = append(f.sources[source], value)
	f.tops[source]++
	return nil
}

func (f *FusedStack[T]) Pop(source int) (T, error) {
	var zero T
	if err := f.validSource(source); err != nil {
		return zero, err
	}
	if f.tops[source] == 0 {
		return zero, fmt.Errorf("source %d stack is empty", source)
	}
	f.tops[source]--
	value := f.sources[source][f.tops[source]]
	f.sources[source] = f.sources[source][:f.tops[source]]
	f.fusion[f.tops[source]] ^= value
	for len(f.fusion) > 0 && f.fusion[len(f.fusion)-1] == 0 && f.maxTop() < len(f.fusion) {
		f.fusion = f.fusion[:len(f.fusion)-1]
	}
	return value, nil
}

func (f *FusedStack[T]) Recover(missing int, survivors map[int][]T) ([]T, error) {
	if err := f.validSource(missing); err != nil {
		return nil, err
	}
	if len(survivors) != len(f.sources)-1 {
		return nil, fmt.Errorf("need %d survivors, got %d", len(f.sources)-1, len(survivors))
	}
	out := make([]T, f.tops[missing])
	copy(out, f.fusion)
	for i, source := range survivors {
		if i < 0 || i >= len(f.sources) || i == missing {
			return nil, fmt.Errorf("invalid survivor %d", i)
		}
		for depth := 0; depth < len(source) && depth < len(out); depth++ {
			out[depth] ^= source[depth]
		}
	}
	return out, nil
}

func (f *FusedStack[T]) Fusion() []T { return append([]T(nil), f.fusion...) }

func (f *FusedStack[T]) Source(i int) []T {
	if f.validSource(i) != nil {
		return nil
	}
	return append([]T(nil), f.sources[i]...)
}

func (f *FusedStack[T]) maxTop() int {
	max := 0
	for _, top := range f.tops {
		if top > max {
			max = top
		}
	}
	return max
}

func (f *FusedStack[T]) validSource(source int) error {
	if source < 0 || source >= len(f.sources) {
		return fmt.Errorf("source %d out of range", source)
	}
	return nil
}
