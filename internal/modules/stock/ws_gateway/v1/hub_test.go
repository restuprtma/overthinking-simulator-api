package v1

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"tuai/pkg/logger"
)

// TestMain initializes the global logger so Hub methods that log via
// logger.Log don't NPE inside tests.
func TestMain(m *testing.M) {
	_ = logger.Initialize("development")
	os.Exit(m.Run())
}

// fakeChannel records every call and returns whatever the test pre-loads.
// Used to verify the Hub routes commands to the right channel.
type fakeChannel struct {
	name string

	startCalls  atomic.Int32
	stopCalls   atomic.Int32
	subCalls    atomic.Int32
	unsubCalls  atomic.Int32
	resyncCalls atomic.Int32
	discCalls   atomic.Int32

	lastSub     string
	lastSubErr  error
}

func (f *fakeChannel) Name() string { return f.name }
func (f *fakeChannel) Start(_ context.Context) error {
	f.startCalls.Add(1)
	return nil
}
func (f *fakeChannel) Stop() { f.stopCalls.Add(1) }
func (f *fakeChannel) HandleSubscribe(c *Client, raw json.RawMessage) error {
	f.subCalls.Add(1)
	f.lastSub = string(raw)
	return f.lastSubErr
}
func (f *fakeChannel) HandleUnsubscribe(c *Client, raw json.RawMessage) error {
	f.unsubCalls.Add(1)
	return nil
}
func (f *fakeChannel) HandleResync(c *Client, raw json.RawMessage) error {
	f.resyncCalls.Add(1)
	return nil
}
func (f *fakeChannel) OnDisconnect(c *Client) { f.discCalls.Add(1) }

// mkClient builds a Client suitable for tests — no real websocket.Conn,
// but the send channel works so Send() returns true/false correctly.
func mkClient(hub *Hub, id string) *Client {
	c := &Client{
		hub:  hub,
		send: make(chan []byte, 16),
		id:   id,
	}
	hub.mu.Lock()
	hub.clients[c] = true
	hub.mu.Unlock()
	return c
}

// drainOne reads one buffered Send. Times out cleanly if nothing's there.
func drainOne(t *testing.T, c *Client) []byte {
	t.Helper()
	select {
	case b := <-c.send:
		return b
	case <-time.After(50 * time.Millisecond):
		t.Fatalf("expected a send to client %q, got nothing", c.id)
		return nil
	}
}

func TestHubRegistersChannelsAndStartsThem(t *testing.T) {
	h := NewHub(DefaultHubConfig())
	ch1 := &fakeChannel{name: "orderbook"}
	ch2 := &fakeChannel{name: "candle"}
	h.Register(ch1)
	h.Register(ch2)

	names := h.Channels()
	if len(names) != 2 {
		t.Fatalf("Channels(): got %d want 2 (%v)", len(names), names)
	}

	if err := h.Start(context.Background()); err != nil {
		t.Fatalf("Start: %v", err)
	}
	if ch1.startCalls.Load() != 1 || ch2.startCalls.Load() != 1 {
		t.Fatalf("Start calls: ob=%d ca=%d (want 1/1)",
			ch1.startCalls.Load(), ch2.startCalls.Load())
	}

	h.Stop()
	if ch1.stopCalls.Load() != 1 || ch2.stopCalls.Load() != 1 {
		t.Fatalf("Stop calls: ob=%d ca=%d (want 1/1)",
			ch1.stopCalls.Load(), ch2.stopCalls.Load())
	}
}

func TestHubRejectsDuplicateChannelName(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatalf("expected panic on duplicate channel name, got none")
		}
	}()
	h := NewHub(DefaultHubConfig())
	h.Register(&fakeChannel{name: "orderbook"})
	h.Register(&fakeChannel{name: "orderbook"}) // boom
}

func TestHubDispatchSubscribeRoutesToCorrectChannel(t *testing.T) {
	h := NewHub(DefaultHubConfig())
	ob := &fakeChannel{name: "orderbook"}
	ca := &fakeChannel{name: "candle"}
	h.Register(ob)
	h.Register(ca)
	c := mkClient(h, "c1")

	h.dispatchCommand(c, []byte(`{"op":"subscribe","channel":"orderbook","stocks":["BBCA"]}`))
	if ob.subCalls.Load() != 1 || ca.subCalls.Load() != 0 {
		t.Fatalf("subscribe routing: ob=%d ca=%d (want 1/0)",
			ob.subCalls.Load(), ca.subCalls.Load())
	}
	if !strings.Contains(ob.lastSub, `"stocks":["BBCA"]`) {
		t.Fatalf("channel received wrong raw: %s", ob.lastSub)
	}
}

func TestHubDispatchPingReturnsPong(t *testing.T) {
	h := NewHub(DefaultHubConfig())
	c := mkClient(h, "c1")

	h.dispatchCommand(c, []byte(`{"op":"ping"}`))
	body := drainOne(t, c)
	var out map[string]any
	if err := json.Unmarshal(body, &out); err != nil {
		t.Fatalf("pong decode: %v", err)
	}
	if out["type"] != TypePong {
		t.Fatalf("pong type: got %v want %s", out["type"], TypePong)
	}
}

func TestHubDispatchUnknownChannelSendsError(t *testing.T) {
	h := NewHub(DefaultHubConfig())
	h.Register(&fakeChannel{name: "candle"})
	c := mkClient(h, "c1")

	h.dispatchCommand(c, []byte(`{"op":"subscribe","channel":"orderbook"}`))
	body := drainOne(t, c)
	var out ErrorMessage
	if err := json.Unmarshal(body, &out); err != nil {
		t.Fatalf("error decode: %v", err)
	}
	if out.Code != ErrUnknownChannel {
		t.Fatalf("error code: got %q want %q", out.Code, ErrUnknownChannel)
	}
}

func TestHubDispatchBadJSONSendsError(t *testing.T) {
	h := NewHub(DefaultHubConfig())
	c := mkClient(h, "c1")

	h.dispatchCommand(c, []byte(`{not json}`))
	body := drainOne(t, c)
	var out ErrorMessage
	if err := json.Unmarshal(body, &out); err != nil {
		t.Fatalf("error decode: %v", err)
	}
	if out.Code != ErrBadJSON {
		t.Fatalf("error code: got %q want %q", out.Code, ErrBadJSON)
	}
}

func TestHubDispatchUnknownOpSendsError(t *testing.T) {
	h := NewHub(DefaultHubConfig())
	h.Register(&fakeChannel{name: "orderbook"})
	c := mkClient(h, "c1")

	h.dispatchCommand(c, []byte(`{"op":"frobnicate","channel":"orderbook"}`))
	body := drainOne(t, c)
	var out ErrorMessage
	if err := json.Unmarshal(body, &out); err != nil {
		t.Fatalf("error decode: %v", err)
	}
	if out.Code != ErrUnknownOp {
		t.Fatalf("error code: got %q want %q", out.Code, ErrUnknownOp)
	}
}

func TestHubUnregisterFiresOnDisconnectForAllChannels(t *testing.T) {
	h := NewHub(DefaultHubConfig())
	ob := &fakeChannel{name: "orderbook"}
	ca := &fakeChannel{name: "candle"}
	h.Register(ob)
	h.Register(ca)
	c := mkClient(h, "c1")

	h.unregister(c)

	if ob.discCalls.Load() != 1 || ca.discCalls.Load() != 1 {
		t.Fatalf("disconnect calls: ob=%d ca=%d (want 1/1)",
			ob.discCalls.Load(), ca.discCalls.Load())
	}
}
