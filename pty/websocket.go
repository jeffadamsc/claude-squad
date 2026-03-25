package pty

import (
	"encoding/json"
	"io"
	"log"
	"net"
	"net/http"
	"strings"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

type resizeMsg struct {
	Type string `json:"type"`
	Rows uint16 `json:"rows"`
	Cols uint16 `json:"cols"`
}

type WebSocketServer struct {
	manager *Manager
	mux     *http.ServeMux
}

func NewWebSocketServer(manager *Manager) *WebSocketServer {
	ws := &WebSocketServer{
		manager: manager,
		mux:     http.NewServeMux(),
	}
	ws.mux.HandleFunc("/ws/", ws.handleWS)
	return ws
}

func (ws *WebSocketServer) Handler() http.Handler {
	return ws.mux
}

func (ws *WebSocketServer) ListenAndServe() (int, error) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	port := listener.Addr().(*net.TCPAddr).Port
	go http.Serve(listener, ws.mux)
	return port, nil
}

func (ws *WebSocketServer) handleWS(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/ws/"), "/")
	if len(parts) == 0 || parts[0] == "" {
		http.Error(w, "missing session ID", http.StatusBadRequest)
		return
	}
	sessionID := parts[0]

	sess := ws.manager.Get(sessionID)
	if sess == nil {
		http.Error(w, "session not found", http.StatusNotFound)
		return
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("websocket upgrade error: %v", err)
		return
	}
	defer conn.Close()

	done := make(chan struct{})
	go func() {
		defer close(done)
		buf := make([]byte, 32*1024)
		for {
			n, err := sess.Read(buf)
			if n > 0 {
				if writeErr := conn.WriteMessage(websocket.BinaryMessage, buf[:n]); writeErr != nil {
					return
				}
			}
			if err != nil {
				return
			}
		}
	}()

	go func() {
		for {
			msgType, msg, err := conn.ReadMessage()
			if err != nil {
				return
			}
			if msgType == websocket.TextMessage {
				var rm resizeMsg
				if json.Unmarshal(msg, &rm) == nil && rm.Type == "resize" {
					ws.manager.Resize(sessionID, rm.Rows, rm.Cols)
				}
				continue
			}
			if _, err := sess.Write(msg); err != nil {
				if err == io.ErrClosedPipe {
					return
				}
			}
		}
	}()

	<-done
}
