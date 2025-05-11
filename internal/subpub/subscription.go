package subpub

import "sync"

type subscriber struct {
	subpub   *subpubImpl
	subject  string
	ch       chan any
	cb       MessageHandler
	closed   chan struct{}
	unsubbed bool
	mu       sync.Mutex
}

func (s *subscriber) Unsubscribe() {
	s.mu.Lock()
	if s.unsubbed {
		s.mu.Unlock()
		return
	}
	s.unsubbed = true
	s.mu.Unlock()

	close(s.closed)
	s.subpub.Lock()
	defer s.subpub.Unlock()

	delete(s.subpub.subscribers[s.subject], s)
	if len(s.subpub.subscribers[s.subject]) == 0 {
		delete(s.subpub.subscribers, s.subject)
	}
}
