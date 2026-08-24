package httpapi

import "net/http"

func (s *Server) publishSnapshot(w http.ResponseWriter, r *http.Request) {
	sn, err := s.app.Snapshots.Publish(r.PathValue("id"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, sn)
}

func (s *Server) listSnapshots(w http.ResponseWriter, r *http.Request) {
	snapshots, err := s.app.Snapshots.List(r.PathValue("id"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, snapshots)
}

func (s *Server) getSnapshot(w http.ResponseWriter, r *http.Request) {
	sn, err := s.app.Snapshots.Get(r.PathValue("id"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, sn)
}
