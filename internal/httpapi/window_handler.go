package httpapi

import "net/http"

type ingestWindowReq struct {
	TriggerIndex int64     `json:"trigger_index"`
	StartTimeNs  int64     `json:"start_time_ns"`
	DurationNs   int64     `json:"duration_ns"`
	Samples      []float64 `json:"samples"`
}

func (s *Server) ingestWindow(w http.ResponseWriter, r *http.Request) {
	var req ingestWindowReq
	if err := decode(r, &req); err != nil {
		writeError(w, err)
		return
	}
	run, err := s.app.Runs.Get(r.PathValue("id"))
	if err != nil {
		writeError(w, err)
		return
	}
	res, err := s.app.Windows.Ingest(run, req.TriggerIndex, req.StartTimeNs, req.DurationNs, req.Samples)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, res)
}

func (s *Server) listWindows(w http.ResponseWriter, r *http.Request) {
	windows, err := s.app.Windows.ListWindows(r.PathValue("id"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, windows)
}

func (s *Server) markSaturated(w http.ResponseWriter, r *http.Request) {
	win, err := s.app.Windows.MarkSaturated(r.PathValue("id"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, win)
}
