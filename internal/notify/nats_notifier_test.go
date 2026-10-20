package notify

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"net"
	"strconv"
	"strings"
	"testing"
	"time"
)

// fakeNATS speaks just enough of the NATS wire protocol for nats.go to
// connect, then reports every PUB it receives.
type fakeNATS struct {
	ln   net.Listener
	pubs chan pub
}

type pub struct {
	subject string
	payload []byte
}

func startFakeNATS(t *testing.T) *fakeNATS {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	f := &fakeNATS{ln: ln, pubs: make(chan pub, 8)}
	t.Cleanup(func() { _ = ln.Close() })
	go f.serve()
	return f
}

func (f *fakeNATS) url() string { return "nats://" + f.ln.Addr().String() }

func (f *fakeNATS) serve() {
	conn, err := f.ln.Accept()
	if err != nil {
		return
	}
	defer conn.Close()
	_, _ = conn.Write([]byte(`INFO {"server_id":"fake","version":"2.10.0","max_payload":1048576,"proto":1}` + "\r\n"))
	r := bufio.NewReader(conn)
	for {
		line, err := r.ReadString('\n')
		if err != nil {
			return
		}
		line = strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(line, "PING"):
			_, _ = conn.Write([]byte("PONG\r\n"))
		case strings.HasPrefix(line, "PUB "):
			fields := strings.Fields(line)
			size, err := strconv.Atoi(fields[len(fields)-1])
			if err != nil {
				return
			}
			buf := make([]byte, size+2)
			if _, err := readFull(r, buf); err != nil {
				return
			}
			f.pubs <- pub{subject: fields[1], payload: buf[:size]}
		}
	}
}

func readFull(r *bufio.Reader, buf []byte) (int, error) {
	n := 0
	for n < len(buf) {
		m, err := r.Read(buf[n:])
		n += m
		if err != nil {
			return n, err
		}
	}
	return n, nil
}

func sampleTransition() VerdictTransition {
	return VerdictTransition{
		Namespace:        "shop",
		Name:             "checkout",
		TargetDeployment: "checkout-api",
		OldVerdict:       "Healthy",
		NewVerdict:       "Degraded",
		Message:          "error rate above threshold",
		Timestamp:        time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC),
	}
}

func TestNATSNotifier_PublishesTransitionAsJSON(t *testing.T) {
	srv := startFakeNATS(t)
	n, err := NewNATSNotifier(srv.url())
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer n.Close()

	want := sampleTransition()
	if err := n.NotifyVerdictTransition(context.Background(), want); err != nil {
		t.Fatalf("notify: %v", err)
	}
	if err := n.conn.Flush(); err != nil {
		t.Fatalf("flush: %v", err)
	}

	select {
	case got := <-srv.pubs:
		if got.subject != "checkoutgate.shop.checkout.verdict" {
			t.Fatalf("subject = %q", got.subject)
		}
		var decoded VerdictTransition
		if err := json.Unmarshal(got.payload, &decoded); err != nil {
			t.Fatalf("payload is not JSON: %v", err)
		}
		if decoded != want {
			t.Fatalf("payload = %+v, want %+v", decoded, want)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("no message reached the server")
	}
}

func TestNATSNotifier_CancelledContextSkipsPublish(t *testing.T) {
	srv := startFakeNATS(t)
	n, err := NewNATSNotifier(srv.url())
	if err != nil {
		t.Fatal(err)
	}
	defer n.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := n.NotifyVerdictTransition(ctx, sampleTransition()); !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want context.Canceled", err)
	}
	_ = n.conn.Flush()
	select {
	case p := <-srv.pubs:
		t.Fatalf("unexpected publish to %q", p.subject)
	case <-time.After(200 * time.Millisecond):
	}
}

func TestNATSNotifier_PublishAfterCloseFails(t *testing.T) {
	srv := startFakeNATS(t)
	n, err := NewNATSNotifier(srv.url())
	if err != nil {
		t.Fatal(err)
	}
	n.conn.Close()
	err = n.NotifyVerdictTransition(context.Background(), sampleTransition())
	if err == nil || !strings.Contains(err.Error(), "publishing to") {
		t.Fatalf("err = %v, want a wrapped publish error", err)
	}
}

func TestNewNATSNotifier_ConnectError(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := ln.Addr().String()
	_ = ln.Close()

	_, err = NewNATSNotifier("nats://" + addr)
	if err == nil || !strings.Contains(err.Error(), "connecting to NATS") {
		t.Fatalf("err = %v, want a wrapped connect error", err)
	}
}

func TestNATSNotifier_CloseWithoutConnIsSafe(t *testing.T) {
	(&NATSNotifier{}).Close()
}
