package transport

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"configcenter/internal/event"
	"configcenter/internal/service"
)

func (s *Server) subscribe(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, r, fmt.Errorf("streaming unsupported"))
		return
	}
	app := r.PathValue("app")
	environment := r.PathValue("env")
	app = strings.ToLower(app)
	token := service.ConstantTimeToken(r.Header.Get("Authorization"))
	release, err := s.configurations.Current(r.Context(), app, environment, token)
	if err != nil {
		writeError(w, r, err)
		return
	}
	subscription, replay, complete := s.hub.Subscribe(app, environment, r.Header.Get("Last-Event-ID"))
	defer subscription.Close()
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")
	ready := event.Event{Application: app, Environment: environment, Version: release.Version,
		Checksum: release.Checksum, Operation: "ready", CreatedAt: time.Now().UTC()}
	if err := sendSSE(w, "ready", ready); err != nil {
		return
	}
	if !complete {
		if err := sendSSE(w, "resync", ready); err != nil {
			return
		}
	} else {
		for _, item := range replay {
			if err := sendSSE(w, "config.changed", item); err != nil {
				return
			}
		}
	}
	flusher.Flush()
	ticker := time.NewTicker(s.heartbeat)
	defer ticker.Stop()
	for {
		select {
		case <-r.Context().Done():
			return
		case <-ticker.C:
			if _, err := fmt.Fprintf(w, ": heartbeat %d\n\n", time.Now().Unix()); err != nil {
				return
			}
			flusher.Flush()
		case item, open := <-subscription.Events:
			if !open {
				return
			}
			if err := sendSSE(w, "config.changed", item); err != nil {
				return
			}
			flusher.Flush()
		}
	}
}

func sendSSE(w http.ResponseWriter, name string, item event.Event) error {
	data, err := json.Marshal(item)
	if err != nil {
		return err
	}
	if item.ID != "" {
		if _, err := fmt.Fprintf(w, "id: %s\n", item.ID); err != nil {
			return err
		}
	}
	_, err = fmt.Fprintf(w, "event: %s\ndata: %s\n\n", name, data)
	return err
}
