package docker

import (
	"context"
	"strconv"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/events"
	"github.com/docker/docker/client"
)

const (
	watcherDefaultBufsiz = 20
	watcherActionStart   = "start"
	watcherActionDie     = "die"
)

// Watcher watches for "start", "die" container
// events in Docker and exposes them via the Events()
// method.
type Watcher interface {
	Events() <-chan WatchEvent
	Shutdown()
	Err() error
	Done() <-chan struct{}
}

type WatchEvent struct {
	Type EventType
	ID   string
}

func NewWatcher(ctx context.Context, client *client.Client) Watcher {
	ctx, cancel := context.WithCancel(ctx)

	w := &watcher{
		client:  client,
		eventch: make(chan WatchEvent, watcherDefaultBufsiz),
		donech:  make(chan struct{}),
		cancel:  cancel,
		ctx:     ctx,
	}

	go w.run()

	return w
}

type watcher struct {
	client  *client.Client
	eventch chan WatchEvent
	donech  chan struct{}
	err     error
	cancel  context.CancelFunc
	ctx     context.Context
}

func (w *watcher) Events() <-chan WatchEvent {
	return w.eventch
}

func (w *watcher) Shutdown() {
	w.cancel()
	<-w.donech
}

func (w *watcher) Done() <-chan struct{} {
	return w.donech
}

func (w *watcher) Err() error {
	<-w.donech
	return w.err
}

func (w *watcher) run() {
	defer close(w.donech)

	options := types.EventsOptions{}

	eventch, errch := w.client.Events(w.ctx, options)

loop:
	for {
		select {

		case <-w.ctx.Done():
			break loop

		case <-errch:

			// todo: throttle retries, die after x consecutive

			eventch, errch = w.client.Events(w.ctx, options)

		case event := <-eventch:

			options.Since = strconv.FormatInt(event.Time, 10)

			if !watcherAcceptEvent(event) {
				continue
			}

			var wevent WatchEvent

			switch event.Action {
			case watcherActionStart:
				wevent = WatchEvent{EventTypeCreate, event.Actor.ID}
			case watcherActionDie:
				wevent = WatchEvent{EventTypeDelete, event.Actor.ID}
			}

			select {
			case w.eventch <- wevent:
			default:
				// todo: warn dropping events
			}

		}
	}

	w.cancel()

	<-errch

}

func watcherAcceptEvent(event events.Message) bool {
	switch {
	case event.Type != events.ContainerEventType:
		return false
	case event.Scope != "local":
		return false
	case event.Actor.ID == "":
		return false
	case event.Action == watcherActionStart:
		return true
	case event.Action == watcherActionDie:
		return true
	default:
		return false
	}
}
