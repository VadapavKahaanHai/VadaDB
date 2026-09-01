package structures

import (
	"fmt"

	"vadadb/core"
)

type FusedArray[T core.Word] struct {
	sources      [][]T
	fusion       []T
	lengthParity uint
}

func NewFusedArray[T core.Word](sources ...[]T) (*FusedArray[T], error) {
	if len(sources) < 2 {
		return nil, fmt.Errorf("fusion needs at least two sources")
	}
	f := &FusedArray[T]{sources: make([][]T, len(sources))}
	for i, source := range sources {
		f.sources[i] = append([]T(nil), source...)
		f.lengthParity ^= uint(len(source))
	}
	f.fusion = core.XORSlices(f.sources...)
	return f, nil
}

func (f *FusedArray[T]) Set(source, index int, value T) error {
	if source < 0 || source >= len(f.sources) {
		return fmt.Errorf("source %d out of range", source)
	}
	if index < 0 || index >= len(f.sources[source]) {
		return fmt.Errorf("index %d out of range", index)
	}
	old := f.sources[source][index]
	f.sources[source][index] = value
	f.fusion[index] = core.Replace(f.fusion[index], old, value)
	return nil
}

func (f *FusedArray[T]) Fusion() []T { return append([]T(nil), f.fusion...) }

func (f *FusedArray[T]) Source(i int) []T {
	if i < 0 || i >= len(f.sources) {
		return nil
	}
	return append([]T(nil), f.sources[i]...)
}

func (f *FusedArray[T]) Recover(missing int, survivors map[int][]T) ([]T, error) {
	if missing < 0 || missing >= len(f.sources) {
		return nil, fmt.Errorf("source %d out of range", missing)
	}
	if len(survivors) != len(f.sources)-1 {
		return nil, fmt.Errorf("need %d survivors, got %d", len(f.sources)-1, len(survivors))
	}
	length := f.lengthParity
	for i, source := range survivors {
		if i < 0 || i >= len(f.sources) || i == missing {
			return nil, fmt.Errorf("invalid survivor %d", i)
		}
		length ^= uint(len(source))
	}
	if length > uint(len(f.fusion)) {
		return nil, fmt.Errorf("recovered length %d exceeds fusion", length)
	}
	out := append([]T(nil), f.fusion[:length]...)
	for _, source := range survivors {
		for i := 0; i < len(source) && i < len(out); i++ {
			out[i] ^= source[i]
		}
	}
	return out, nil
}
