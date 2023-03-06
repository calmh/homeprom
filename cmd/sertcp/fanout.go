package main

import (
	"sync"
)

const buffer = 16

type fanout[T any] struct {
	mut  sync.Mutex
	subs []chan<- T
}

func NewFanout[T any]() *fanout[T] {
	return &fanout[T]{}
}

func (s *fanout[T]) Publish(val T) error {
	s.mut.Lock()
	for _, sub := range s.subs {
		select {
		case sub <- val:
		default:
		}
	}
	s.mut.Unlock()
	return nil
}

func (s *fanout[T]) Listen() *fanoutSub[T] {
	ch := make(chan T, buffer)
	s.mut.Lock()
	s.subs = append(s.subs, ch)
	s.mut.Unlock()
	return &fanoutSub[T]{s, ch}
}

func (s *fanout[T]) release(ch chan<- T) {
	s.mut.Lock()
	defer s.mut.Unlock()
	for i, sub := range s.subs {
		if sub == ch {
			s.subs = append(s.subs[:i], s.subs[i+1:]...)
			return
		}
	}
}

type fanoutSub[T any] struct {
	pubsub *fanout[T]
	ch     chan T
}

func (s *fanoutSub[T]) Channel() <-chan T {
	return s.ch
}

func (s *fanoutSub[T]) Close() error {
	s.pubsub.release(s.ch)
	return nil
}
