package httpapi

import "net/http"

func (s *Server) getBaseline(w http.ResponseWriter, r *http.Request) {
	b, err := s.app.Deconv.GetBaseline(r.PathValue("id"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, b)
}

type lockReferenceReq struct {
	WindowID string `json:"window_id"`
}

func (s *Server) lockReference(w http.ResponseWriter, r *http.Request) {
	var req lockReferenceReq
	if err := decode(r, &req); err != nil {
		writeError(w, err)
		return
	}
	ref, err := s.app.Deconv.LockReference(r.PathValue("id"), req.WindowID)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, ref)
}
