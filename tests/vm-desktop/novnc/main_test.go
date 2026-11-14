package main

import (
	"io"
	"net"
	"net/http/httptest"
	"strings"
	"testing"

	"golang.org/x/net/websocket"
)

// echoServer stands in for x11vnc: it echoes every byte back.
func echoServer(t *testing.T) string {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { l.Close() })
	go func() {
		for {
			c, err := l.Accept()
			if err != nil {
				return
			}
			go func() { _, _ = io.Copy(c, c); c.Close() }()
		}
	}()
	return l.Addr().String()
}

func TestBridgeRelaysBothWays(t *testing.T) {
	srv := httptest.NewServer(newMux(t.TempDir(), echoServer(t)))
	defer srv.Close()
	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http") + "/websockify"
	cfg, err := websocket.NewConfig(wsURL, srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	cfg.Protocol = []string{"binary"}
	ws, err := websocket.DialConfig(cfg)
	if err != nil {
		t.Fatalf("same-origin dial refused: %v", err)
	}
	defer ws.Close()
	if _, err := ws.Write([]byte("RFB 003.008\n")); err != nil {
		t.Fatal(err)
	}
	got := make([]byte, 12)
	if _, err := io.ReadFull(ws, got); err != nil {
		t.Fatal(err)
	}
	if string(got) != "RFB 003.008\n" {
		t.Fatalf("relayed %q", got)
	}
}

func TestBridgeRefusesOtherOrigin(t *testing.T) {
	srv := httptest.NewServer(newMux(t.TempDir(), echoServer(t)))
	defer srv.Close()
	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http") + "/websockify"
	if ws, err := websocket.Dial(wsURL, "", "http://evil.example"); err == nil {
		ws.Close()
		t.Fatal("cross-origin WebSocket accepted")
	}
}
