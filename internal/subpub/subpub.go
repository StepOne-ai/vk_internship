package subpub

import (
	"context"
	"errors"
	"sync"
)

var (
	ErrClosed = errors.New("subpub is closed")
)

type MessageHandler func(msg any)

type Subscription interface {
	Unsubscribe()
}

type SubPub interface {
	Subscribe(subject string, cb MessageHandler) (Subscription, error)
	Publish(subject string, msg any) error
	Close(ctx context.Context) error
}

func NewSubPub() SubPub {
	return &subpubImpl{
		subscribers: make(map[string]map[*subscriber]struct{}),
		quit:        make(chan struct{}),
	}
}

type subpubImpl struct {
	sync.RWMutex
	subscribers map[string]map[*subscriber]struct{}
	quit        chan struct{}
	wg          sync.WaitGroup
}

func (s *subpubImpl) Subscribe(subject string, cb MessageHandler) (Subscription, error) {
	s.Lock()
	defer s.Unlock()

	select {
	case <-s.quit:
		return nil, ErrClosed
	default:
	}

	sub := &subscriber{
		subpub:  s,
		subject: subject,
		ch:      make(chan any, 100),
		cb:      cb,
		closed:  make(chan struct{}),
	}

	if _, exists := s.subscribers[subject]; !exists {
		s.subscribers[subject] = make(map[*subscriber]struct{})
	}
	s.subscribers[subject][sub] = struct{}{}

	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		for {
			select {
			case msg := <-sub.ch:
				sub.cb(msg)
			case <-sub.closed:
				return
			}
		}
	}()

	return sub, nil
}

func (s *subpubImpl) Publish(subject string, msg any) error {
	s.RLock()
	defer s.RUnlock()

	select {
	case <-s.quit:
		return ErrClosed
	default:
	}

	subscribers, exists := s.subscribers[subject]
	if !exists {
		return nil
	}

	for sub := range subscribers {
		select {
		case sub.ch <- msg:
		default:
		}
	}

	return nil
}

func (s *subpubImpl) Close(ctx context.Context) error {
	s.Lock()
	select {
	case <-s.quit:
		s.Unlock()
		return ErrClosed
	default:
		close(s.quit)
		var subs []*subscriber
		for _, subMap := range s.subscribers {
			for sub := range subMap {
				subs = append(subs, sub)
			}
		}
		s.Unlock()

		for _, sub := range subs {
			sub.Unsubscribe()
		}

		done := make(chan struct{})
		go func() {
			s.wg.Wait()
			close(done)
		}()

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-done:
			return nil
		}
	}
}
