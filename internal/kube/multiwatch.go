package kube

import (
	"context"
	"sync"
)

type (
	// MultiWatcher allows to watch multiple clusters in the same channel
	MultiWatcher struct {
		watchers []*Watcher
	}
)

// NewMultiWatcher creates a watcher for multiple configs into one chan
func NewMultiWatcher(configs []Config) (mw *MultiWatcher, err error) {
	mw = &MultiWatcher{
		[]*Watcher{},
	}
	for _, c := range configs {
		w, err := NewWatcher(c)
		mw.watchers = append(mw.watchers, w)
		if err != nil {
			return nil, err
		}
	}
	return
}

// Watch nonblocking all Watchers and throw them into a single mw.Sink
func (mw *MultiWatcher) Watch(ctx context.Context) {
	wg := sync.WaitGroup{}
	wg.Add(len(mw.watchers))
	for _, w := range mw.watchers {
		go func(w *Watcher) {
			defer wg.Done()
			w.Watch(ctx)
		}(w)
	}
	wg.Wait()
}

// List availibe Routes
func (mw *MultiWatcher) List() (routes []*Route) {
	routes = []*Route{}
	for _, w := range mw.watchers {
		routes = append(routes, w.List()...)
	}
	return
}
