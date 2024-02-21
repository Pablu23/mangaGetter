package main

type Option[T any] struct {
	Value *T
	Set   bool
}

func Ok[T any](value T) Option[T] {
	return Option[T]{
		Value: &value,
		Set:   true,
	}
}

func None[T any]() Option[T] {
	return Option[T]{
		Value: nil,
		Set:   false,
	}
}
