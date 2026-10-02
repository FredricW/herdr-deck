package herdr

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

// Findings from herdr 0.9.3 (2026-10-02), which shape Watch:
//
//   - After events.subscribe the server replies {"result":{"type":
//     "subscription_started"}} and then keeps the connection open, streaming
//     one {"event", "data"} line per event. Event names use underscores
//     (pane_updated) where subscription types use dots (pane.updated).
//   - pane.updated fires for every pane each time herdr-projects' ticker
//     rewrites its tokens (about every 15 s) and carries the whole pane,
//     agent_status included.
//   - pane.agent_status_changed needs a pane_id: one subscription per pane.
//   - One bad subscription (an unknown pane id, a missing field) fails the
//     whole request with an error reply, and the server closes the
//     connection. An unknown pane id (code pane_not_found) means a pane
//     closed after the snapshot, so Watch subscribes again at once.

// globalEvents are the subscriptions that need no pane id.
var globalEvents = []string{"pane.created", "pane.closed", "pane.exited", "pane.agent_detected", "pane.updated"}

// resubscribeOn are the events after which the set of panes may have
// changed, so Watch subscribes again with the new set.
var resubscribeOn = map[string]bool{"pane_created": true, "pane_closed": true, "pane_exited": true, "pane_agent_detected": true}

// WatchOptions tunes Watch. Zero values take the defaults.
type WatchOptions struct {
	// Poll is how often changed is called when the socket answers but
	// will not stream events (default 2.5 s).
	Poll time.Duration
	// Retry is how long Watch waits before trying again after the socket
	// could not be reached or the stream ended (default 10 s).
	Retry time.Duration
	// Debounce merges a burst of events into one call, made this long
	// after the burst's first event (default 200 ms).
	Debounce time.Duration
}

func (o *WatchOptions) defaults() {
	if o.Poll == 0 {
		o.Poll = 2500 * time.Millisecond
	}
	if o.Retry == 0 {
		o.Retry = 10 * time.Second
	}
	if o.Debounce == 0 {
		o.Debounce = 200 * time.Millisecond
	}
}

// Watch calls changed whenever herdr's live state may have changed, until ctx
// is done. It subscribes to pane events and to every agent pane's status
// changes; when herdr refuses the subscription or its replies cannot be
// read, it polls instead, calling changed every Poll. Without a socket it
// waits and tries again, calling nothing: the deck's own tick still reloads.
// It only reads and subscribes.
func (c Client) Watch(ctx context.Context, opt WatchOptions, changed func()) {
	opt.defaults()
	events := make(chan struct{}, 1)
	go debounce(ctx, events, opt.Debounce, changed)
	notify := func() {
		select {
		case events <- struct{}{}:
		default:
		}
	}
	for ctx.Err() == nil {
		err := c.stream(ctx, notify)
		switch {
		case ctx.Err() != nil:
			return
		case errors.Is(err, errResubscribe):
			sleep(ctx, opt.Debounce) // let a burst of pane changes settle
			continue
		case errors.Is(err, ErrNoSocket):
			sleep(ctx, opt.Retry)
		case errors.Is(err, errNoStream):
			// The server answers but does not stream: poll for a while,
			// then try subscribing again.
			deadline := time.Now().Add(opt.Retry * 3)
			for ctx.Err() == nil && time.Now().Before(deadline) {
				notify()
				sleep(ctx, opt.Poll)
			}
		default:
			// The stream ended (herdr restarted?): reload once, retry soon.
			notify()
			sleep(ctx, opt.Poll)
		}
	}
}

var (
	errResubscribe = errors.New("panes changed")
	errNoStream    = errors.New("herdr does not stream events")
)

// stream subscribes once and reads events until the connection ends, ctx is
// done, or the set of panes changes.
func (c Client) stream(ctx context.Context, notify func()) error {
	st, err := c.Snapshot(ctx)
	if err != nil {
		if errors.Is(err, ErrNoSocket) {
			return err
		}
		return fmt.Errorf("%w: %v", errNoStream, err)
	}
	conn, err := c.dial(ctx)
	if err != nil {
		return err
	}
	defer conn.Close()
	stop := context.AfterFunc(ctx, func() { conn.Close() })
	defer stop()

	if err := send(conn, "deck-events", "events.subscribe", map[string]any{"subscriptions": subscriptions(st)}); err != nil {
		return err
	}
	sc := scanner(conn)
	started := false
	for sc.Scan() {
		var rep reply
		if err := json.Unmarshal(sc.Bytes(), &rep); err != nil {
			continue // an unknown line shape is skipped, not fatal
		}
		switch {
		case rep.Error != nil && rep.Error.Code == "pane_not_found":
			// A pane closed between the snapshot and the subscribe.
			return errResubscribe
		case rep.Error != nil:
			return fmt.Errorf("%w: %s", errNoStream, rep.Error.Message)
		case rep.ID != "":
			started = true
		case rep.Event != "":
			notify()
			if resubscribeOn[rep.Event] {
				return errResubscribe
			}
		}
	}
	if !started {
		return errNoStream
	}
	if err := sc.Err(); err != nil {
		return err
	}
	return errors.New("event stream closed")
}

// subscriptions are the global pane events plus one status subscription per
// pane that runs an agent.
func subscriptions(st State) []map[string]string {
	subs := make([]map[string]string, 0, len(globalEvents)+len(st.Panes))
	for _, t := range globalEvents {
		subs = append(subs, map[string]string{"type": t})
	}
	for _, p := range st.Panes {
		if p.Agent != "" {
			subs = append(subs, map[string]string{"type": "pane.agent_status_changed", "pane_id": p.ID})
		}
	}
	return subs
}

// debounce calls changed d after the first of a burst of events, so a burst
// causes one call and a steady stream cannot hold calls back.
func debounce(ctx context.Context, events <-chan struct{}, d time.Duration, changed func()) {
	timer := time.NewTimer(d)
	timer.Stop()
	armed := false
	for {
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-events:
			if !armed {
				timer.Reset(d)
				armed = true
			}
		case <-timer.C:
			armed = false
			changed()
		}
	}
}

func sleep(ctx context.Context, d time.Duration) {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
	case <-t.C:
	}
}
