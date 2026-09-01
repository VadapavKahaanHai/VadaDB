package core

func XOR[T Word](values ...T) (out T) {
	for _, value := range values {
		out ^= value
	}
	return out
}

func Replace[T Word](fusion, oldValue, newValue T) T {
	return fusion ^ oldValue ^ newValue
}

func Recover[T Word](fusion T, survivors ...T) T {
	return fusion ^ XOR(survivors...)
}

func XORSlices[T Word](sources ...[]T) []T {
	max := 0
	for _, source := range sources {
		if len(source) > max {
			max = len(source)
		}
	}
	out := make([]T, max)
	for _, source := range sources {
		for i, value := range source {
			out[i] ^= value
		}
	}
	return out
}
