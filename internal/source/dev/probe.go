package dev

import (
	"context"
	"net"
	"strconv"
	"sync"
	"time"
)

// ProbeTimeout bounds one port probe: a TCP connect to 127.0.0.1.
const ProbeTimeout = 400 * time.Millisecond

// probeTTL is how long a probe's answer is reused. The deck reloads every
// few seconds and on every file change; a server's state rarely changes
// faster than that.
const probeTTL = 2 * time.Second

// Prober tells whether something listens on local ports, remembering each
// answer for a short while.
type Prober struct {
	// Dial connects to addr, or fails; nil uses a TCP dial with
	// ProbeTimeout. Tests replace it.
	Dial func(ctx context.Context, addr string) error
	// Now is the clock for the cache; nil means time.Now.
	Now func() time.Time

	mu    sync.Mutex
	cache map[int]probe
}

type probe struct {
	up bool
	at time.Time
}

// Listening probes the ports in parallel, those not probed in the last
// moment, and reports which listen. It takes at most about ProbeTimeout.
func (p *Prober) Listening(ctx context.Context, ports []int) map[int]bool {
	now := p.now()
	out := make(map[int]bool, len(ports))
	var todo []int
	p.mu.Lock()
	for _, port := range ports {
		if _, dup := out[port]; dup {
			continue
		}
		if c, ok := p.cache[port]; ok && now.Sub(c.at) < probeTTL {
			out[port] = c.up
			continue
		}
		out[port] = false
		todo = append(todo, port)
	}
	p.mu.Unlock()

	var wg sync.WaitGroup
	var mu sync.Mutex
	for _, port := range todo {
		wg.Add(1)
		go func() {
			defer wg.Done()
			up := p.dial(ctx, net.JoinHostPort("127.0.0.1", strconv.Itoa(port))) == nil
			mu.Lock()
			out[port] = up
			mu.Unlock()
		}()
	}
	wg.Wait()

	p.mu.Lock()
	if p.cache == nil {
		p.cache = map[int]probe{}
	}
	for _, port := range todo {
		p.cache[port] = probe{up: out[port], at: now}
	}
	p.mu.Unlock()
	return out
}

func (p *Prober) dial(ctx context.Context, addr string) error {
	if p.Dial != nil {
		return p.Dial(ctx, addr)
	}
	ctx, cancel := context.WithTimeout(ctx, ProbeTimeout)
	defer cancel()
	var d net.Dialer
	conn, err := d.DialContext(ctx, "tcp", addr)
	if err != nil {
		return err
	}
	return conn.Close()
}

func (p *Prober) now() time.Time {
	if p.Now != nil {
		return p.Now()
	}
	return time.Now()
}
