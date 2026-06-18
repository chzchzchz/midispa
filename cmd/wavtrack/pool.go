package main

import (
	"context"
)

type Pool[T any] struct {
	c chan T
}

func NewPool[T any](sz int) *Pool[T] {
	return &Pool[T]{
		c: make(chan T, sz),
	}
}

func (p *Pool[T]) Put(v T) { p.c <- v }

func (p *Pool[T]) Get() T {
	var v T
	select {
	case v = <-p.c:
		return v
	default:
		return v
	}
}

func (p *Pool[T]) Wait(ctx context.Context) T {
	var v T
	select {
	case v = <-p.c:
		return v
	case <-ctx.Done():
		return v
	}
}
