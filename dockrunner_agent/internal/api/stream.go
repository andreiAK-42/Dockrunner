package api

import (
	"net/http"

	"github.com/coder/websocket"
)

// serveStream отдаёт историю построчно, затем живые сообщения, пока клиент на связи.
func serveStream(w http.ResponseWriter, r *http.Request, history []string, subscribe func() (<-chan string, func())) {
	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{OriginPatterns: []string{"*"}})
	if err != nil {
		return
	}
	defer func() { _ = conn.Close(websocket.StatusNormalClosure, "") }()
	ctx := r.Context()

	for _, line := range history {
		if err := conn.Write(ctx, websocket.MessageText, []byte(line)); err != nil {
			return
		}
	}

	ch, unsubscribe := subscribe()
	defer unsubscribe()
	for {
		select {
		case <-ctx.Done():
			return
		case msg, ok := <-ch:
			if !ok {
				return
			}
			if err := conn.Write(ctx, websocket.MessageText, []byte(msg)); err != nil {
				return
			}
		}
	}
}
