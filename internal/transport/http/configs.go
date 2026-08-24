package transport

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"configcenter/internal/domain"
	"configcenter/internal/service"
)

type draftRequest struct {
	Revision int64               `json:"revision"`
	Items    []domain.ConfigItem `json:"items"`
}

func (s *Server) getDraft(w http.ResponseWriter, r *http.Request) {
	draft, err := s.configurations.Draft(r.Context(), r.PathValue("app"), r.PathValue("env"))
	respond(w, r, http.StatusOK, draft, err)
}

func (s *Server) saveDraft(w http.ResponseWriter, r *http.Request) {
	var input draftRequest
	if err := decodeJSON(w, r, s.maxBodyBytes, &input); err != nil {
		writeError(w, r, err)
		return
	}
	if input.Items == nil {
		input.Items = []domain.ConfigItem{}
	}
	draft, err := s.configurations.SaveDraft(r.Context(), r.PathValue("app"), r.PathValue("env"),
		input.Revision, input.Items, operator(r), requestID(r))
	respond(w, r, http.StatusOK, draft, err)
}

func (s *Server) diffDraft(w http.ResponseWriter, r *http.Request) {
	diff, err := s.configurations.DiffDraft(r.Context(), r.PathValue("app"), r.PathValue("env"))
	respond(w, r, http.StatusOK, diff, err)
}

func (s *Server) clientConfig(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()
	token := service.ConstantTimeToken(r.Header.Get("Authorization"))
	release, err := s.configurations.Current(ctx, r.PathValue("app"), r.PathValue("env"), token)
	if err != nil {
		writeError(w, r, err)
		return
	}
	etag := fmt.Sprintf(`"v%d-%s"`, release.Version, release.Checksum)
	if strings.TrimSpace(r.Header.Get("If-None-Match")) == etag {
		w.WriteHeader(http.StatusNotModified)
		return
	}
	w.Header().Set("ETag", etag)
	w.Header().Set("Cache-Control", "no-cache, private")
	writeJSON(w, http.StatusOK, map[string]any{
		"version": release.Version, "checksum": release.Checksum,
		"config": release.Content, "published_at": release.CreatedAt,
	})
}
