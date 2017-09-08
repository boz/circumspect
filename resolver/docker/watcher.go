package docker

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/docker/engine-api/client"
	"github.com/docker/engine-api/types"
	"github.com/docker/engine-api/types/events"
	"github.com/docker/engine-api/types/filters"
	"github.com/sirupsen/logrus"
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

func NewWatcher(ctx context.Context, client *client.Client, filter filters.Args) Watcher {
	ctx, cancel := context.WithCancel(ctx)

	w := &watcher{
		client:  client,
		filter:  filter,
		eventch: make(chan WatchEvent, watcherDefaultBufsiz),
		donech:  make(chan struct{}),
		log:     log.WithField("component", "watcher"),
		cancel:  cancel,
		ctx:     ctx,
	}

	go w.run()

	return w
}

type watcher struct {
	client  *client.Client
	filter  filters.Args
	eventch chan WatchEvent
	donech  chan struct{}
	err     error
	log     logrus.FieldLogger
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
	defer w.log.Debug("done")
	defer w.cancel()

	options := types.EventsOptions{
		Filters: w.filter,
	}

	// todo: retry, throttle
	stream, err := w.client.Events(w.ctx, options)
	if err != nil {
		log.WithError(err).Error("error getting events")
		return
	}
	defer stream.Close()

	var event events.Message

	decoder := json.NewDecoder(stream)

	for w.ctx.Err() == nil {

		if err := decoder.Decode(&event); err != nil {
			if w.ctx.Err() == nil {
				log.WithError(err).Error("error decoding stream")
			}
			return
		}

		options.Since = fmt.Sprintf("%d.%09d", event.Time, event.TimeNano)

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
			log.Warn("dropping event")
		}
	}
}

func watcherAcceptEvent(event events.Message) bool {
	switch {
	case event.Type != events.ContainerEventType:
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
