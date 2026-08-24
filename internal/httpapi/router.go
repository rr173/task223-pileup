// Package httpapi 提供 HTTP 层：路由注册、请求解析与 JSON 响应。
package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"

	"task223-pileup/internal/model"
	"task223-pileup/internal/service"
)

// Server HTTP 服务器。
type Server struct {
	app *service.App
}

// New 构造 HTTP 服务器。
func New(app *service.App) *Server { return &Server{app: app} }

// Handler 注册全部路由并返回处理器。
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()

	// 运行生命周期。
	mux.HandleFunc("POST /api/runs", s.createRun)
	mux.HandleFunc("GET /api/runs", s.listRuns)
	mux.HandleFunc("GET /api/runs/{id}", s.getRun)
	mux.HandleFunc("POST /api/runs/{id}/finish", s.finishRun)
	mux.HandleFunc("POST /api/runs/{id}/confirm", s.confirmRun)

	// 波形窗口。
	mux.HandleFunc("POST /api/runs/{id}/windows", s.ingestWindow)
	mux.HandleFunc("GET /api/runs/{id}/windows", s.listWindows)
	mux.HandleFunc("POST /api/windows/{id}/saturate", s.markSaturated)

	// 基线估计与参考脉冲。
	mux.HandleFunc("GET /api/runs/{id}/baseline", s.getBaseline)
	mux.HandleFunc("POST /api/runs/{id}/reference", s.lockReference)

	// 解卷积。
	mux.HandleFunc("POST /api/runs/{id}/deconvolve", s.deconvolveRun)

	// 脉冲与死区。
	mux.HandleFunc("GET /api/runs/{id}/pulses", s.listPulses)
	mux.HandleFunc("GET /api/pulses/{id}", s.getPulse)
	mux.HandleFunc("POST /api/pulses/{id}/confirm", s.confirmPulse)
	mux.HandleFunc("POST /api/pulses/{id}/reject", s.rejectPulse)
	mux.HandleFunc("GET /api/runs/{id}/deadzones", s.listDeadZones)

	// 计数快照。
	mux.HandleFunc("POST /api/runs/{id}/snapshots", s.publishSnapshot)
	mux.HandleFunc("GET /api/runs/{id}/snapshots", s.listSnapshots)
	mux.HandleFunc("GET /api/snapshots/{id}", s.getSnapshot)

	// 统计与健康。
	mux.HandleFunc("GET /api/stats", s.stats)
	mux.HandleFunc("GET /api/health", s.health)

	return mux
}

func (s *Server) health(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok", "service": "task223-pileup"})
}

func (s *Server) stats(w http.ResponseWriter, r *http.Request) {
	st, err := s.app.Stats()
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, st)
}

// writeJSON 输出 JSON 响应。
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// writeError 把领域错误映射为 HTTP 状态码并输出。
func writeError(w http.ResponseWriter, err error) {
	status := http.StatusInternalServerError
	switch {
	case errors.Is(err, model.ErrNotFound):
		status = http.StatusNotFound
	case errors.Is(err, model.ErrInvalidInput):
		status = http.StatusBadRequest
	case errors.Is(err, model.ErrInvalidState), errors.Is(err, model.ErrConflict):
		status = http.StatusConflict
	case errors.Is(err, model.ErrSealed):
		status = http.StatusConflict
	case errors.Is(err, model.ErrDuplicate):
		status = http.StatusConflict
	case errors.Is(err, model.ErrInsufficientData):
		status = http.StatusUnprocessableEntity
	case errors.Is(err, model.ErrDeconvFailed):
		status = http.StatusUnprocessableEntity
	}
	writeJSON(w, status, map[string]string{"error": err.Error()})
}

// decode 解析 JSON 请求体。
func decode(r *http.Request, v any) error {
	defer r.Body.Close()
	return json.NewDecoder(r.Body).Decode(v)
}
