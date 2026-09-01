package core

// Fusible is Definition 1's recovery contract. Implementations also maintain
// their fusion incrementally and expose their own structure-specific updates.
type Fusible[T any] interface {
	Recover(missing int, survivors map[int]T) (T, error)
}

// Word is the set of values supported by the paper's XOR fusion operator.
type Word interface {
	~uint | ~uint8 | ~uint16 | ~uint32 | ~uint64
}
