// Package herdr reads herdr's live state over its Unix socket: the panes, the
// agents in them and their tokens, and a stream of events that says when
// they change. Against herdr it only reads and subscribes; the commands that
// act are Focus, which the deck runs when the user presses enter, and
// RunInPane, which opens a terminal editor or diff tool beside it. The
// herdr plugin hooks (internal/plugin) send their few commands through Call.
//
// The socket speaks newline-delimited JSON: a request is
// {"id", "method", "params"} and its reply {"id", "result"} or
// {"id", "error"}. `herdr api schema --json` lists every method.
package herdr

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"syscall"
	"time"
)

// EnvSocket names the environment variable herdr sets to its socket path.
const EnvSocket = "HERDR_SOCKET_PATH"

// callTimeout bounds one request/reply round trip.
const callTimeout = 3 * time.Second

// maxLine is the longest reply line read; a snapshot of a busy session is
// about 50 KB.
const maxLine = 16 << 20

// SocketPath returns $HERDR_SOCKET_PATH, else ~/.config/herdr/herdr.sock.
func SocketPath(getenv func(string) string) string {
	if p := getenv(EnvSocket); p != "" {
		return p
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".config", "herdr", "herdr.sock")
}

// Pane is one herdr pane as session.snapshot describes it.
type Pane struct {
	ID          string            `json:"pane_id"`
	WorkspaceID string            `json:"workspace_id"`
	TabID       string            `json:"tab_id"`
	Cwd         string            `json:"cwd"`
	Agent       string            `json:"agent"`        // e.g. "claude", or "" for a plain shell
	AgentStatus string            `json:"agent_status"` // idle, working, blocked, done, unknown
	Tokens      map[string]string `json:"tokens"`       // e.g. hp_project, hp_group, hp_sub
}

// Workspace is one herdr workspace as session.snapshot describes it.
type Workspace struct {
	ID       string            `json:"workspace_id"`
	Tokens   map[string]string `json:"tokens"`   // e.g. port, set by a worktree plugin
	Worktree *Checkout         `json:"worktree"` // the git checkout the workspace was opened on, if any
}

// Checkout is the git checkout a workspace was opened on.
type Checkout struct {
	Path string `json:"checkout_path"`
}

// State is the part of herdr's live session the deck uses.
type State struct {
	Panes      []Pane      `json:"panes"`
	Workspaces []Workspace `json:"workspaces"`
}

// ErrNoSocket says nothing listens on the socket path: herdr is not running,
// or runs with another socket.
var ErrNoSocket = errors.New("herdr socket not found")

// Client talks to the herdr server on Socket.
type Client struct {
	Socket string
}

// Snapshot reads the live session (method session.snapshot, the same data
// as `herdr api snapshot`).
func (c Client) Snapshot(ctx context.Context) (State, error) {
	var res struct {
		Snapshot State `json:"snapshot"`
	}
	if err := c.call(ctx, "session.snapshot", struct{}{}, &res); err != nil {
		return State{}, err
	}
	return res.Snapshot, nil
}

// Focus focuses the pane, switching to its workspace and tab (method
// pane.focus).
func (c Client) Focus(ctx context.Context, paneID string) error {
	return c.call(ctx, "pane.focus", map[string]string{"pane_id": paneID}, nil)
}

// RunInPane splits a new pane below target, focuses it and types line into
// its shell, followed by Enter (methods pane.split and pane.send_input). The
// new pane starts in cwd.
func (c Client) RunInPane(ctx context.Context, target, cwd, line string) error {
	return c.OpenPane(ctx, target, cwd, line, false)
}

// OpenPane is RunInPane that, with zoom set, also zooms the new pane to
// the whole tab before typing (pane.zoom, mode on), so a view that wants
// room gets it; herdr's zoom key gives the other panes back. A zoom that
// fails leaves the pane as it is.
func (c Client) OpenPane(ctx context.Context, target, cwd, line string, zoom bool) error {
	var res struct {
		Pane struct {
			ID string `json:"pane_id"`
		} `json:"pane"`
	}
	err := c.call(ctx, "pane.split", map[string]any{
		"target_pane_id": target,
		"direction":      "down",
		"cwd":            cwd,
		"focus":          true,
	}, &res)
	if err != nil {
		return err
	}
	if res.Pane.ID == "" {
		return errors.New("pane.split returned no pane id")
	}
	if zoom {
		_ = c.call(ctx, "pane.zoom", map[string]any{"pane_id": res.Pane.ID, "mode": "on"}, nil)
	}
	return c.call(ctx, "pane.send_input", map[string]any{
		"pane_id": res.Pane.ID,
		"text":    line,
		"keys":    []string{"Enter"},
	}, nil)
}

// Call sends one request and decodes its reply's result into out (unless
// out is nil). A herdr error reply is returned as an error.
func (c Client) Call(ctx context.Context, method string, params, out any) error {
	return c.call(ctx, method, params, out)
}

func (c Client) dial(ctx context.Context) (net.Conn, error) {
	if c.Socket == "" {
		return nil, ErrNoSocket
	}
	var d net.Dialer
	conn, err := d.DialContext(ctx, "unix", c.Socket)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) || errors.Is(err, syscall.ENOENT) || errors.Is(err, syscall.ECONNREFUSED) {
			return nil, fmt.Errorf("%w at %s", ErrNoSocket, c.Socket)
		}
		return nil, err
	}
	return conn, nil
}

// call sends one request and decodes its reply's result into out (unless
// out is nil).
func (c Client) call(ctx context.Context, method string, params, out any) error {
	ctx, cancel := context.WithTimeout(ctx, callTimeout)
	defer cancel()
	conn, err := c.dial(ctx)
	if err != nil {
		return err
	}
	defer conn.Close()
	if dl, ok := ctx.Deadline(); ok {
		_ = conn.SetDeadline(dl)
	}
	if err := send(conn, "deck", method, params); err != nil {
		return err
	}
	sc := scanner(conn)
	if !sc.Scan() {
		if err := sc.Err(); err != nil {
			return fmt.Errorf("%s: %w", method, err)
		}
		return fmt.Errorf("%s: no reply", method)
	}
	var rep reply
	if err := json.Unmarshal(sc.Bytes(), &rep); err != nil {
		return fmt.Errorf("%s: %w", method, err)
	}
	if rep.Error != nil {
		return fmt.Errorf("%s: %s", method, rep.Error.Message)
	}
	if out == nil {
		return nil
	}
	if err := json.Unmarshal(rep.Result, out); err != nil {
		return fmt.Errorf("%s: %w", method, err)
	}
	return nil
}

type reply struct {
	ID     string          `json:"id"`
	Result json.RawMessage `json:"result"`
	Error  *struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
	// Events have these instead of an id.
	Event string          `json:"event"`
	Data  json.RawMessage `json:"data"`
}

func send(conn net.Conn, id, method string, params any) error {
	b, err := json.Marshal(map[string]any{"id": id, "method": method, "params": params})
	if err != nil {
		return err
	}
	_, err = conn.Write(append(b, '\n'))
	return err
}

func scanner(conn net.Conn) *bufio.Scanner {
	sc := bufio.NewScanner(conn)
	sc.Buffer(make([]byte, 0, 64<<10), maxLine)
	return sc
}
