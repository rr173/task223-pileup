package httpapi

import "net/http"

func (s *Server) listPulses(w http.ResponseWriter, r *http.Request) {
	pulses, err := s.app.Deconv.ListPulses(r.PathValue("id"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, pulses)
}

func (s *Server) getPulse(w http.ResponseWriter, r *http.Request) {
	p, err := s.app.Deconv.GetPulse(r.PathValue("id"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, p)
}

func (s *Server) confirmPulse(w http.ResponseWriter, r *http.Request) {
	p, err := s.app.Deconv.ConfirmPulse(r.PathValue("id"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, p)
}

func (s *Server) rejectPulse(w http.ResponseWriter, r *http.Request) {
	p, err := s.app.Deconv.RejectPulse(r.PathValue("id"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, p)
}

func (s *Server) listDeadZones(w http.ResponseWriter, r *http.Request) {
	zones, err := s.app.Deconv.ListDeadZones(r.PathValue("id"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, zones)
}
