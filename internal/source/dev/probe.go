package dev

import (
	"context"
	"net"
	"net/http"
	"strconv"
	"sync"
	"time"
)

// ProbeTimeout bounds one port probe: a TCP connect to 127.0.0.1 and to
// ::1, at the same time.
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
	// Get asks for url and returns its HTTP status; nil uses a GET with
	// HTTPTimeout. Tests replace it.
	Get func(ctx context.Context, url string) (int, error)
	// Now is the clock for the cache; nil means time.Now.
	Now func() time.Time

	mu    sync.Mutex
	cache map[int]probe
	http  map[check]probe
}

// HTTPTimeout bounds a readiness GET (spec section 6.1).
const HTTPTimeout = 2 * time.Second

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
		// A server may listen on IPv4 or IPv6 only (Node resolves
		// localhost to ::1 first), and a localhost URL reaches either.
		for _, host := range []string{"127.0.0.1", "::1"} {
			wg.Add(1)
			go func() {
				defer wg.Done()
				if p.dial(ctx, net.JoinHostPort(host, strconv.Itoa(port))) == nil {
					mu.Lock()
					out[port] = true
					mu.Unlock()
				}
			}()
		}
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

// Ready runs the checks that ask over HTTP, in parallel, those not asked
// in the last moment, and reports which answered 200–399. Checks without a
// path are Listening's.
func (p *Prober) Ready(ctx context.Context, checks []check) map[check]bool {
	now := p.now()
	out := map[check]bool{}
	var todo []check
	p.mu.Lock()
	for _, c := range checks {
		if _, dup := out[c]; dup || c.Path == "" || c.Port == 0 {
			continue
		}
		if r, ok := p.http[c]; ok && now.Sub(r.at) < probeTTL {
			out[c] = r.up
			continue
		}
		out[c] = false
		todo = append(todo, c)
	}
	p.mu.Unlock()
	var wg sync.WaitGroup
	var mu sync.Mutex
	for _, c := range todo {
		wg.Go(func() {
			ok := p.ask(ctx, c)
			mu.Lock()
			out[c] = ok
			mu.Unlock()
		})
	}
	wg.Wait()
	p.mu.Lock()
	if p.http == nil {
		p.http = map[check]probe{}
	}
	for _, c := range todo {
		p.http[c] = probe{up: out[c], at: now}
	}
	p.mu.Unlock()
	return out
}

// Check runs one check now, without the cache: while the deck waits for a
// service to get ready.
func (p *Prober) Check(ctx context.Context, c check) bool {
	if c.Port == 0 {
		return false
	}
	if c.Path != "" {
		return p.ask(ctx, c)
	}
	for _, host := range []string{"127.0.0.1", "::1"} {
		if p.dial(ctx, net.JoinHostPort(host, strconv.Itoa(c.Port))) == nil {
			return true
		}
	}
	return false
}

// ask GETs the check's path on 127.0.0.1, then on ::1.
func (p *Prober) ask(ctx context.Context, c check) bool {
	for _, host := range []string{"127.0.0.1", "[::1]"} {
		url := "http://" + host + ":" + strconv.Itoa(c.Port) + c.Path
		if code, err := p.get(ctx, url); err == nil && code >= 200 && code < 400 {
			return true
		}
	}
	return false
}

func (p *Prober) get(ctx context.Context, url string) (int, error) {
	if p.Get != nil {
		return p.Get(ctx, url)
	}
	ctx, cancel := context.WithTimeout(ctx, HTTPTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return 0, err
	}
	// Redirects count as an answer: 3xx is ready.
	client := http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	resp, err := client.Do(req)
	if err != nil {
		return 0, err
	}
	resp.Body.Close()
	return resp.StatusCode, nil
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
