package transport

import (
	"context"
	"embed"
	"io/fs"
	"log/slog"
	"net/http"
	"time"

	"configcenter/internal/event"
	"configcenter/internal/repository"
	"configcenter/internal/service"
)

//go:embed web/*
var webFiles embed.FS

type Server struct {
	repository     repository.Repository
	applications   *service.Applications
	configurations *service.Configurations
	releases       *service.Releases
	hub            *event.Hub
	adminToken     string
	heartbeat      time.Duration
	maxBodyBytes   int64
	logger         *slog.Logger
	mux            *http.ServeMux
}

func NewServer(repo repository.Repository, hub *event.Hub, adminToken string, heartbeat time.Duration, maxBodyBytes int64, logger *slog.Logger) *Server {
	applications := service.NewApplications(repo)
	server := &Server{
		repository: repo, applications: applications,
		configurations: service.NewConfigurations(repo, applications),
		releases:       service.NewReleases(repo, applications, hub),
		hub:            hub, adminToken: adminToken, heartbeat: heartbeat,
		maxBodyBytes: maxBodyBytes, logger: logger, mux: http.NewServeMux(),
	}
	server.routes()
	return server
}

func (s *Server) Handler() http.Handler {
	return s.requestID(s.recoverPanic(s.requestLog(s.securityHeaders(s.mux))))
}

func (s *Server) routes() {
	s.mux.HandleFunc("GET /healthz", s.health)
	s.mux.HandleFunc("GET /readyz", s.ready)
	s.mux.Handle("POST /api/v1/apps", s.admin(http.HandlerFunc(s.createApplication)))
	s.mux.Handle("GET /api/v1/apps", s.admin(http.HandlerFunc(s.listApplications)))
	s.mux.Handle("GET /api/v1/apps/{app}", s.admin(http.HandlerFunc(s.getApplication)))
	s.mux.Handle("POST /api/v1/apps/{app}/token/reset", s.admin(http.HandlerFunc(s.resetToken)))
	s.mux.Handle("POST /api/v1/apps/{app}/envs", s.admin(http.HandlerFunc(s.createEnvironment)))
	s.mux.Handle("GET /api/v1/apps/{app}/envs", s.admin(http.HandlerFunc(s.listEnvironments)))
	s.mux.Handle("GET /api/v1/apps/{app}/envs/{env}/draft", s.admin(http.HandlerFunc(s.getDraft)))
	s.mux.Handle("PUT /api/v1/apps/{app}/envs/{env}/draft", s.admin(http.HandlerFunc(s.saveDraft)))
	s.mux.Handle("GET /api/v1/apps/{app}/envs/{env}/diff", s.admin(http.HandlerFunc(s.diffDraft)))
	s.mux.Handle("POST /api/v1/apps/{app}/envs/{env}/releases", s.admin(http.HandlerFunc(s.publish)))
	s.mux.Handle("GET /api/v1/apps/{app}/envs/{env}/releases", s.admin(http.HandlerFunc(s.listReleases)))
	s.mux.Handle("GET /api/v1/apps/{app}/envs/{env}/releases/{version}", s.admin(http.HandlerFunc(s.getRelease)))
	s.mux.Handle("GET /api/v1/apps/{app}/envs/{env}/compare", s.admin(http.HandlerFunc(s.compareReleases)))
	s.mux.Handle("POST /api/v1/apps/{app}/envs/{env}/rollback", s.admin(http.HandlerFunc(s.rollback)))
	s.mux.Handle("GET /api/v1/audits", s.admin(http.HandlerFunc(s.listAudits)))
	s.mux.HandleFunc("GET /client/v1/apps/{app}/envs/{env}/config", s.clientConfig)
	s.mux.HandleFunc("GET /client/v1/apps/{app}/envs/{env}/subscribe", s.subscribe)
	assets, _ := fs.Sub(webFiles, "web")
	s.mux.Handle("GET /", spaHandler(assets))
}

func (s *Server) health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) ready(w http.ResponseWriter, r *http.Request) {
	if err := s.repository.Ping(context.Background()); err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ready"})
}

func spaHandler(files fs.FS) http.Handler {
	server := http.FileServer(http.FS(files))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			data, _ := fs.ReadFile(files, "index.html")
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			_, _ = w.Write(data)
			return
		}
		server.ServeHTTP(w, r)
	})
}
