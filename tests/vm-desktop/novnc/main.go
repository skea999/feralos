// novnc serves the noVNC web client and bridges its WebSocket to the VNC
// server: the Go stand-in for the Python websockify (no Python in FeralOS
// tools). Reached from the owner's browser through `box forward`.
package main

import (
	"flag"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"

	"golang.org/x/net/websocket"
)

func main() {
	listen := flag.String("listen", "127.0.0.1:6080", "HTTP address")
	web := flag.String("web", "/usr/share/novnc", "noVNC files")
	vnc := flag.String("vnc", "127.0.0.1:5900", "VNC server")
	flag.Parse()
	log.Printf("noVNC on http://%s/vnc.html, bridging to %s", *listen, *vnc)
	log.Fatal(http.ListenAndServe(*listen, newMux(*web, *vnc)))
}

func newMux(web, vnc string) *http.ServeMux {
	mux := http.NewServeMux()
	mux.Handle("/", http.FileServer(http.Dir(web)))
	mux.Handle("/websockify", bridge(vnc))
	return mux
}

// bridge relays binary WebSocket frames to a TCP connection and back.
func bridge(vnc string) websocket.Server {
	return websocket.Server{
		Handshake: func(c *websocket.Config, r *http.Request) error {
			// Same origin only: any page open in the browser could otherwise
			// drive the VM desktop through ws://localhost.
			if o, err := url.Parse(r.Header.Get("Origin")); err != nil || o.Host != r.Host {
				return websocket.ErrBadWebSocketOrigin
			}
			// noVNC may offer the "binary" subprotocol; echo it when offered.
			for _, p := range c.Protocol {
				if p == "binary" {
					c.Protocol = []string{"binary"}
					return nil
				}
			}
			c.Protocol = nil
			return nil
		},
		Handler: func(ws *websocket.Conn) {
			defer ws.Close()
			ws.PayloadType = websocket.BinaryFrame
			tcp, err := net.Dial("tcp", vnc)
			if err != nil {
				log.Print(err)
				return
			}
			defer tcp.Close()
			done := make(chan struct{}, 2)
			go func() { _, _ = io.Copy(tcp, ws); done <- struct{}{} }()
			go func() { _, _ = io.Copy(ws, tcp); done <- struct{}{} }()
			<-done // either side closing ends the session; the defers close both
		},
	}
}
