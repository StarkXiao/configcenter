package transport

import (
	"net/http"
	"strconv"

	"configcenter/internal/domain"
)

type publishRequest struct {
	ExpectedVersion int64  `json:"expected_version"`
	Summary         string `json:"summary"`
}

type rollbackRequest struct {
	ExpectedVersion int64  `json:"expected_version"`
	TargetVersion   int64  `json:"target_version"`
	Reason          string `json:"reason"`
}

func (s *Server) publish(w http.ResponseWriter, r *http.Request) {
	var input publishRequest
	if err := decodeJSON(w, r, s.maxBodyBytes, &input); err != nil {
		writeError(w, r, err)
		return
	}
	release, err := s.releases.Publish(r.Context(), r.PathValue("app"), r.PathValue("env"),
		input.Summary, operator(r), requestID(r), input.ExpectedVersion)
	respond(w, r, http.StatusCreated, release, err)
}

func (s *Server) rollback(w http.ResponseWriter, r *http.Request) {
	var input rollbackRequest
	if err := decodeJSON(w, r, s.maxBodyBytes, &input); err != nil {
		writeError(w, r, err)
		return
	}
	release, err := s.releases.Rollback(r.Context(), r.PathValue("app"), r.PathValue("env"),
		input.TargetVersion, input.Reason, operator(r), requestID(r), input.ExpectedVersion)
	respond(w, r, http.StatusCreated, release, err)
}

func (s *Server) listReleases(w http.ResponseWriter, r *http.Request) {
	limit := queryInt(r, "limit", 20)
	offset := queryInt(r, "offset", 0)
	page, err := s.releases.List(r.Context(), r.PathValue("app"), r.PathValue("env"), limit, offset)
	respond(w, r, http.StatusOK, page, err)
}

func (s *Server) getRelease(w http.ResponseWriter, r *http.Request) {
	version, err := strconv.ParseInt(r.PathValue("version"), 10, 64)
	if err != nil || version <= 0 {
		writeError(w, r, domain.NewError(domain.CodeInvalid, "version must be a positive integer"))
		return
	}
	release, err := s.releases.Get(r.Context(), r.PathValue("app"), r.PathValue("env"), version)
	if err != nil {
		writeError(w, r, err)
		return
	}
	for index := range release.Items {
		if release.Items[index].Sensitive {
			release.Items[index].Value = "******"
		}
	}
	release.Content = nil
	writeJSON(w, http.StatusOK, release)
}

func (s *Server) compareReleases(w http.ResponseWriter, r *http.Request) {
	from := queryInt64(r, "from")
	to := queryInt64(r, "to")
	if from <= 0 || to <= 0 || from == to {
		writeError(w, r, domain.NewError(domain.CodeInvalid, "from and to must be different positive versions"))
		return
	}
	diff, err := s.releases.Compare(r.Context(), r.PathValue("app"), r.PathValue("env"), from, to)
	respond(w, r, http.StatusOK, diff, err)
}

func (s *Server) listAudits(w http.ResponseWriter, r *http.Request) {
	limit := queryInt(r, "limit", 20)
	if limit > 100 {
		limit = 100
	}
	items, err := s.repository.ListAudits(r.Context(), limit, queryInt(r, "offset", 0))
	respond(w, r, http.StatusOK, map[string]any{"items": items}, err)
}

func queryInt(r *http.Request, name string, fallback int) int {
	value, err := strconv.Atoi(r.URL.Query().Get(name))
	if err != nil {
		return fallback
	}
	return value
}

func queryInt64(r *http.Request, name string) int64 {
	value, _ := strconv.ParseInt(r.URL.Query().Get(name), 10, 64)
	return value
}
