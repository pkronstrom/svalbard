package frontpage

import (
	"context"
	"io"
	"sync"

	"github.com/pkronstrom/svalbard/drive-runtime/internal/settings"
	"github.com/pkronstrom/svalbard/drive-runtime/internal/wiki"
)

// wikiService shares the same SilverBullet editor between the Wiki and
// Serve Everything actions. It stays open until both release it.
type wikiService struct {
	mu      sync.Mutex
	current *wikiLease
}

type wikiLease struct {
	instance *wiki.Instance
	users    int
	done     chan struct{}
	released chan struct{}
	err      error // written before done is closed
}

func (s *wikiService) acquire(ctx context.Context, root string) (*wikiLease, func(), error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return nil, nil, err
	}
	lease := s.current
	if lease == nil {
		instance, err := wiki.Start(ctx, root, settings.OpenForAction(root, io.Discard))
		if err != nil {
			return nil, nil, err
		}
		lease = &wikiLease{instance: instance, done: make(chan struct{}), released: make(chan struct{})}
		s.current = lease
		go s.watch(lease)
	}
	lease.users++
	return lease, func() {
		s.mu.Lock()
		defer s.mu.Unlock()
		lease.users--
		if s.current == lease && lease.users == 0 {
			s.current = nil
			close(lease.released)
			lease.instance.Close()
		}
	}, nil
}

func (s *wikiService) watch(lease *wikiLease) {
	select {
	case err := <-lease.instance.Exited():
		s.mu.Lock()
		defer s.mu.Unlock()
		lease.err = err
		close(lease.done)
		if s.current == lease {
			s.current = nil
			lease.instance.Close()
		}
	case <-lease.released:
	}
}
