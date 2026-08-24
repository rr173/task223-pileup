package httpapi

import "net/http"

type createRunReq struct {
	Name         string  `json:"name"`
	Description  string  `json:"description"`
	DetectorType string  `json:"detector_type"`
	SampleRateHz float64 `json:"sample_rate_hz"`
	DeadTimeNs   int64   `json:"dead_time_ns"`
}

func (s *Server) createRun(w http.ResponseWriter, r *http.Request) {
	var req createRunReq
	if err := decode(r, &req); err != nil {
		writeError(w, err)
		return
	}
	run, err := s.app.Runs.Create(req.Name, req.Description, req.DetectorType, req.SampleRateHz, req.DeadTimeNs)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, run)
}

func (s *Server) listRuns(w http.ResponseWriter, r *http.Request) {
	runs, err := s.app.Runs.List()
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, runs)
}

func (s *Server) getRun(w http.ResponseWriter, r *http.Request) {
	run, err := s.app.Runs.Get(r.PathValue("id"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, run)
}

func (s *Server) finishRun(w http.ResponseWriter, r *http.Request) {
	run, err := s.app.Runs.FinishReceiving(r.PathValue("id"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, run)
}

func (s *Server) confirmRun(w http.ResponseWriter, r *http.Request) {
	run, err := s.app.Runs.Confirm(r.PathValue("id"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, run)
}

func (s *Server) deconvolveRun(w http.ResponseWriter, r *http.Request) {
	res, err := s.app.Deconv.Deconvolve(r.PathValue("id"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, res)
}
