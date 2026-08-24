package transport

import "net/http"

type applicationRequest struct {
	Name        string `json:"name"`
	Slug        string `json:"slug"`
	Description string `json:"description"`
}

type environmentRequest struct {
	Name        string `json:"name"`
	Code        string `json:"code"`
	Description string `json:"description"`
}

func (s *Server) createApplication(w http.ResponseWriter, r *http.Request) {
	var input applicationRequest
	if err := decodeJSON(w, r, s.maxBodyBytes, &input); err != nil {
		writeError(w, r, err)
		return
	}
	created, err := s.applications.Create(r.Context(), input.Name, input.Slug, input.Description)
	respond(w, r, http.StatusCreated, created, err)
}

func (s *Server) listApplications(w http.ResponseWriter, r *http.Request) {
	items, err := s.applications.List(r.Context(), r.URL.Query().Get("q"), r.URL.Query().Get("status"))
	respond(w, r, http.StatusOK, map[string]any{"items": items}, err)
}

func (s *Server) getApplication(w http.ResponseWriter, r *http.Request) {
	application, err := s.applications.Get(r.Context(), r.PathValue("app"))
	if err != nil {
		writeError(w, r, err)
		return
	}
	environments, err := s.applications.ListEnvironments(r.Context(), application.Slug)
	respond(w, r, http.StatusOK, map[string]any{"application": application, "environments": environments}, err)
}

func (s *Server) resetToken(w http.ResponseWriter, r *http.Request) {
	token, err := s.applications.ResetToken(r.Context(), r.PathValue("app"), operator(r), requestID(r))
	respond(w, r, http.StatusOK, map[string]string{"access_token": token}, err)
}

func (s *Server) createEnvironment(w http.ResponseWriter, r *http.Request) {
	var input environmentRequest
	if err := decodeJSON(w, r, s.maxBodyBytes, &input); err != nil {
		writeError(w, r, err)
		return
	}
	item, err := s.applications.CreateEnvironment(r.Context(), r.PathValue("app"), input.Name, input.Code, input.Description)
	respond(w, r, http.StatusCreated, item, err)
}

func (s *Server) listEnvironments(w http.ResponseWriter, r *http.Request) {
	items, err := s.applications.ListEnvironments(r.Context(), r.PathValue("app"))
	respond(w, r, http.StatusOK, map[string]any{"items": items}, err)
}
