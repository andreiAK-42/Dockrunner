package worker

import (
	"context"
	"encoding/base64"
	"encoding/json"

	amqp "github.com/rabbitmq/amqp091-go"

	"dockrunner_agent/internal/files"
)

func failResponse(err error) map[string]any {
	return map[string]any{"ok": false, "error": err.Error()}
}

func badContentResponse() map[string]any {
	return map[string]any{"ok": false, "error": "bad content"}
}

func (h *Handler) replyFiles(ctx context.Context, msg amqp.Delivery, task Task) error {
	response := map[string]any{"ok": true}

	switch task.Type {
	case "files.list":
		entries, err := files.List(h.workspace, task.Owner, task.Path)
		if err != nil {
			response = failResponse(err)
			break
		}
		response["entries"] = entries
	case "files.mkdir":
		if err := files.Mkdir(h.workspace, task.Owner, task.Path); err != nil {
			response = failResponse(err)
		}
	case "files.upload":
		data, err := base64.StdEncoding.DecodeString(task.ContentB64)
		if err != nil {
			response = badContentResponse()
			break
		}
		if err := files.Write(h.workspace, task.Owner, task.Path, data); err != nil {
			response = failResponse(err)
		}
	case "files.download":
		data, err := files.Read(h.workspace, task.Owner, task.Path)
		if err != nil {
			response = failResponse(err)
			break
		}
		response["content_b64"] = base64.StdEncoding.EncodeToString(data)
	case "files.delete":
		if err := files.Delete(h.workspace, task.Owner, task.Path); err != nil {
			response = failResponse(err)
		}
	}

	body, _ := json.Marshal(response)
	return h.queue.Reply(ctx, msg, body)
}
