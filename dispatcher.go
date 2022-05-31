package core

import (
	"context"
	"sort"
	"time"

	"github.com/cornelk/hashmap"
	"github.com/pkg/errors"
)

type ErrEventStopped struct{}

func (e ErrEventStopped) Error() string {
	return "Event stopped"
}

type Event interface {
	GetName() string
}

type EventSubscriber func(context.Context, Event) error

type EventSubscribers []EventSubscriber

type EventDispatcher interface {
	Configure(cfg EventDispatcherConfig)
	Subscribe(string, EventSubscriber)
	Dispatch(ctx context.Context, event Event) error
}

type ListenerEntry struct {
	Event      string
	Subscriber EventSubscriber
	Priority   uint8
}

type EventDispatcherConfig []ListenerEntry

type dispatcher struct {
	subscribers hashmap.HashMap
	debug       bool
}

func NewDispatcher(debug bool) EventDispatcher {
	return &dispatcher{
		debug: debug,
	}
}

func (d *dispatcher) Configure(cfg EventDispatcherConfig) {
	sort.SliceStable(cfg, func(i, j int) bool {
		return cfg[i].Priority > cfg[j].Priority
	})
	for _, c := range cfg {
		d.Subscribe(c.Event, c.Subscriber)
	}
}

func (d *dispatcher) Subscribe(evt string, subscriber EventSubscriber) {
	var s = []EventSubscriber{subscriber}
	subs, ok := d.subscribers.Get(evt)
	if ok {
		s = subs.([]EventSubscriber)
		s = append(s, subscriber)
	}
	d.subscribers.Set(evt, s)
}

func (d *dispatcher) Dispatch(ctx context.Context, event Event) error {
	subs, ok := d.subscribers.Get(event.GetName())
	if !ok {
		return nil
	}
	s := subs.([]EventSubscriber)
	if d.debug {
		profile, ok := ctx.Value(profileContextKey).(*Profile)
		if ok {
			start := time.Now()
			defer func(ctx context.Context) {
				profile.AddEventDispatcherProfile(event.GetName(), time.Now().Sub(start).Seconds(), s)
			}(ctx)
		}
	}

	return d.doDispatch(ctx, event, s)
}

func (d *dispatcher) doDispatch(ctx context.Context, event Event, subs []EventSubscriber) error {
	for _, sub := range subs {
		if err := sub(ctx, event); err != nil {
			if errors.As(err, &ErrEventStopped{}) {
				break
			}
			return err
		}
	}
	return nil
}

func dispatchEventSilent(ctx context.Context, dispatcher EventDispatcher, event Event) error {
	if dispatcher == nil {
		return nil
	}
	return dispatcher.Dispatch(ctx, event)
}
