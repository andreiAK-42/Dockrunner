package api

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"sync"

	"dockrunner_agent/internal/logs"
	"dockrunner_agent/internal/store"
)

type LogsResponse struct {
	Data string `json:"data"`
}

// handleContainerLogs godoc
// @Summary Хвост логов контейнера
// @Tags agent
// @Produce json
// @Param Authorization header string false "Bearer <agent_token> (или ?token= в query)"
// @Param id path string true "ID деплоя"
// @Param tail query int false "Число строк" default(200)
// @Success 200 {object} api.LogsResponse
// @Failure 401 {object} api.ErrorResponse
// @Failure 404 {object} api.ErrorResponse
// @Router /containers/{id}/logs [get]
func (s *Server) handleContainerLogs(w http.ResponseWriter, r *http.Request) {
	dep, ok := s.findContainer(r)
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "deployment not found"})
		return
	}
	tail := queryTail(r)
	// Контейнер ещё не создан (идёт сборка): отдаём лог сборки — именно он
	// интересен пользователю в этот момент, а не ошибка Docker.
	if dep.Container == "" {
		for _, id := range s.buildLogSources(dep.ID) {
			if text, found := s.builds.Tail(id, tail); found {
				writeJSON(w, http.StatusOK, LogsResponse{Data: text})
				return
			}
		}
		writeJSON(w, http.StatusOK, LogsResponse{Data: "контейнер ещё не создан — идёт сборка или запуск"})
		return
	}
	// Docker logs are the source of truth. The in-memory streamer only retains
	// a small rolling buffer and must not replace the requested historical tail.
	text, err := s.dock.LogsTail(r.Context(), dep.Container, strconv.Itoa(tail))
	if err == nil {
		writeJSON(w, http.StatusOK, LogsResponse{Data: text})
		return
	}
	// A running stream may still have useful recent output if Docker's log
	// driver does not support reading historical logs.
	history := s.streamer.History(dep.Container, tail)
	if len(history) > 0 {
		lines := make([]string, 0, len(history))
		for _, raw := range history {
			var entry logs.Entry
			if json.Unmarshal([]byte(raw), &entry) == nil {
				lines = append(lines, entry.Line)
			}
		}
		writeJSON(w, http.StatusOK, LogsResponse{Data: strings.Join(lines, "\n")})
		return
	}
	writeJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
}

// handleBuildLog godoc
// @Summary Лог сборки деплоя (шаги Dockerfile, скачивание слоёв)
// @Tags agent
// @Produce json
// @Param Authorization header string false "Bearer <agent_token> (или ?token= в query)"
// @Param id path string true "ID деплоя"
// @Param tail query int false "Число строк" default(200)
// @Success 200 {object} api.LogsResponse
// @Failure 401 {object} api.ErrorResponse
// @Failure 404 {object} api.ErrorResponse
// @Router /deployments/{id}/build-log [get]
func (s *Server) handleBuildLog(w http.ResponseWriter, r *http.Request) {
	tail := queryTail(r)
	// Берём первый непустой источник: для compose-сервиса это лог его
	// собственной сборки, а для старых записей — лог родительского деплоя.
	for _, id := range s.buildLogSources(r.PathValue("id")) {
		if text, ok := s.builds.Tail(id, tail); ok {
			writeJSON(w, http.StatusOK, LogsResponse{Data: text})
			return
		}
	}
	writeJSON(w, http.StatusNotFound, map[string]string{"error": "no build log"})
}

// buildLogSources возвращает id, под которыми может храниться лог сборки.
// Для compose-сервиса панель запрашивает дочерний id, а старые версии агента
// писали лог под id родительского деплоя.
func (s *Server) buildLogSources(id string) []string {
	sources := []string{id}
	dep, err := store.GetDeployment(s.db, id)
	if err == nil && dep.ParentID != "" && dep.ParentID != id {
		sources = append(sources, dep.ParentID)
	}
	return sources
}

// handleBuildLogStream godoc
// @Summary Живой лог сборки по WebSocket
// @Description Сначала отдаёт историю (200 строк), затем живые строки текстом. Токен — в ?token=, заголовок в браузере не поставить.
// @Tags agent
// @Param id path string true "ID деплоя"
// @Param token query string false "Токен агента"
// @Success 101 "Переключение на WebSocket"
// @Failure 401 {object} api.ErrorResponse
// @Failure 404 {object} api.ErrorResponse
// @Router /deployments/{id}/build-log/stream [get]
func (s *Server) handleBuildLogStream(w http.ResponseWriter, r *http.Request) {
	ids := s.buildLogSources(r.PathValue("id"))

	// История — из первого источника, где она есть, чтобы не дублировать
	// строки, если лог лежит и под дочерним, и под родительским id.
	var history []string
	for _, id := range ids {
		if text, ok := s.builds.Tail(id, 10000); ok {
			history = append(history, strings.Split(text, "\n")...)
			break
		}
	}

	serveStream(w, r, history, func() (<-chan string, func()) {
		if len(ids) == 1 {
			return s.builds.Subscribe(ids[0])
		}

		// Несколько id (родитель + сервис): сливаем живые строки в один канал.
		merged := make(chan string, 64)
		done := make(chan struct{})
		unsubs := make([]func(), 0, len(ids))
		var wg sync.WaitGroup

		for _, id := range ids {
			ch, unsub := s.builds.Subscribe(id)
			unsubs = append(unsubs, unsub)
			wg.Add(1)
			go func(ch <-chan string) {
				defer wg.Done()
				for {
					select {
					case <-done:
						return
					case line, ok := <-ch:
						if !ok {
							return
						}
						select {
						case merged <- line:
						case <-done:
							return
						}
					}
				}
			}(ch)
		}

		return merged, func() {
			close(done)
			for _, unsub := range unsubs {
				unsub()
			}
			wg.Wait()
			close(merged)
		}
	})
}

// handleContainerLogStream godoc
// @Summary Живые логи по WebSocket
// @Description Сначала отдаёт историю (200 строк), затем живые сообщения {stream, line, time}. Токен — в ?token=, заголовок в браузере не поставить.
// @Tags agent
// @Param id path string true "ID деплоя"
// @Param token query string false "Токен агента"
// @Success 101 "Переключение на WebSocket"
// @Failure 401 {object} api.ErrorResponse
// @Failure 404 {object} api.ErrorResponse
// @Router /containers/{id}/logs/stream [get]
func (s *Server) handleContainerLogStream(w http.ResponseWriter, r *http.Request) {
	dep, ok := s.findContainer(r)
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "deployment not found"})
		return
	}

	// Стример цепляется к контейнерам при деплое и при старте агента.
	// Восстановленные из Docker деплои (потерянная SQLite) остаются без
	// подписки, и живые строки не текут — цепляемся лениво, Attach идемпотентен.
	if dep.Container != "" {
		s.streamer.Attach(dep.Container, dep.Name)
	}

	var history []string
	if limit := queryHistory(r); limit > 0 {
		history = s.streamer.History(dep.Container, limit)
	}

	serveStream(w, r, history, func() (<-chan string, func()) {
		return s.streamer.Subscribe(dep.Container)
	})
}

// queryTail разбирает ?tail=, по умолчанию 200.
func queryTail(r *http.Request) int {
	raw := r.URL.Query().Get("tail")
	if raw == "" {
		return 200
	}
	tail, err := strconv.Atoi(raw)
	if err != nil || tail <= 0 {
		return 200
	}
	return tail
}

// queryHistory reads ?history=. The backend asks for history=0 because it
// already loads the full tail over HTTP and only needs live lines here.
func queryHistory(r *http.Request) int {
	raw := r.URL.Query().Get("history")
	if raw == "" {
		return 200
	}
	limit, err := strconv.Atoi(raw)
	if err != nil || limit < 0 {
		return 200
	}
	if limit > 500 {
		return 500
	}
	return limit
}
