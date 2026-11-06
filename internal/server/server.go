package server

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Cooper-Wallace/agentkit/internal/caffeine"
)

// ServerConfig configures the loopback HTTP and SSE service.
type ServerConfig struct {
	Host string `json:"host"`
	Port int    `json:"port"`
}

// DefaultServerConfig returns the default configuration binding to 127.0.0.1:18820.
func DefaultServerConfig() ServerConfig {
	return ServerConfig{
		Host: "127.0.0.1",
		Port: 18820,
	}
}

// Server exposes REST and SSE endpoints for the caffeine engine.
type Server struct {
	config     ServerConfig
	engine     *caffeine.Engine
	ownsEngine bool
	httpServer *http.Server
	listener   net.Listener
	mu         sync.Mutex
	running    bool
}

// NewServer creates a new Server instance.
func NewServer(cfg ServerConfig, eng *caffeine.Engine) *Server {
	owns := false
	if eng == nil {
		eng = caffeine.NewEngine(caffeine.DefaultEngineConfig(), nil, nil)
		owns = true
	}
	if cfg.Host == "" {
		cfg.Host = "127.0.0.1"
	}

	s := &Server{
		config:     cfg,
		engine:     eng,
		ownsEngine: owns,
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/status", s.handleStatus)
	mux.HandleFunc("/v1/status", s.handleStatus)
	mux.HandleFunc("/notify", s.handleNotify)
	mux.HandleFunc("/v1/notify", s.handleNotify)
	mux.HandleFunc("/override", s.handleOverride)
	mux.HandleFunc("/v1/override", s.handleOverride)
	mux.HandleFunc("/cancel", s.handleCancel)
	mux.HandleFunc("/v1/cancel", s.handleCancel)
	mux.HandleFunc("/history", s.handleHistory)
	mux.HandleFunc("/v1/history", s.handleHistory)
	mux.HandleFunc("/events", s.handleEvents)

	s.httpServer = &http.Server{
		Handler:      mux,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 0, // SSE streaming requires unbounded write timeout
	}

	return s
}

// Engine returns the underlying caffeine engine.
func (s *Server) Engine() *caffeine.Engine {
	return s.engine
}

// Port returns the bound listening port.
func (s *Server) Port() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.listener != nil {
		return s.listener.Addr().(*net.TCPAddr).Port
	}
	return s.config.Port
}

// Addr returns the complete host:port listening address.
func (s *Server) Addr() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.listener != nil {
		return s.listener.Addr().String()
	}
	return fmt.Sprintf("%s:%d", s.config.Host, s.config.Port)
}

// Start binds to the configured network interface and serves requests in the background.
func (s *Server) Start() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.running {
		return nil
	}

	addr := fmt.Sprintf("%s:%d", s.config.Host, s.config.Port)
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("failed to bind to %s: %w", addr, err)
	}

	s.listener = ln
	s.running = true

	go func() {
		_ = s.httpServer.Serve(ln)
	}()

	return nil
}

// Stop gracefully shuts down the HTTP server and releases resources.
func (s *Server) Stop(ctx context.Context) error {
	s.mu.Lock()
	if !s.running {
		s.mu.Unlock()
		return nil
	}
	s.running = false
	s.mu.Unlock()

	var stopErr error
	if s.httpServer != nil {
		stopErr = s.httpServer.Shutdown(ctx)
	}
	if s.ownsEngine && s.engine != nil {
		_ = s.engine.Close()
	}

	return stopErr
}

// sendJSON helper serializes response body as JSON.
func sendJSON(w http.ResponseWriter, statusCode int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(data)
}

// handleStatus returns current activity, leases, timers, hardware, and breakers.
func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		sendJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	st := s.engine.GetStatus()
	sendJSON(w, http.StatusOK, st)
}

// NotifyPayload defines the JSON body accepted by /notify.
type NotifyPayload struct {
	SessionID string `json:"session_id"`
	Session   string `json:"session"`
	State     string `json:"state"`
	Agent     string `json:"agent"`
	Task      string `json:"task"`
	PID       int    `json:"pid"`
}

// handleNotify processes session state transitions.
func (s *Server) handleNotify(w http.ResponseWriter, r *http.Request) {
	var payload NotifyPayload

	if r.Method == http.MethodPost {
		body, err := io.ReadAll(r.Body)
		if err == nil && len(body) > 0 {
			_ = json.Unmarshal(body, &payload)
		}
	} else if r.Method == http.MethodGet {
		// Fallback for query parameters
		q := r.URL.Query()
		payload.SessionID = q.Get("session_id")
		if payload.SessionID == "" {
			payload.SessionID = q.Get("session")
		}
		payload.State = q.Get("state")
		payload.Agent = q.Get("agent")
		payload.Task = q.Get("task")
		if pidVal, err := strconv.Atoi(q.Get("pid")); err == nil {
			payload.PID = pidVal
		}
	} else {
		sendJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}

	sid := payload.SessionID
	if sid == "" {
		sid = payload.Session
	}

	if sid == "" || payload.State == "" {
		sendJSON(w, http.StatusBadRequest, map[string]string{"error": "missing required fields 'session_id' and 'state'"})
		return
	}

	agent := payload.Agent
	if agent == "" {
		agent = "agent"
	}

	st, err := s.engine.Notify(sid, payload.State, agent, payload.Task, payload.PID)
	if err != nil {
		sendJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	sendJSON(w, http.StatusOK, st)
}

// OverridePayload defines the JSON body accepted by /override.
type OverridePayload struct {
	State           string `json:"state"`
	Enabled         *bool  `json:"enabled"`
	Duration        string `json:"duration"`
	DurationSeconds int    `json:"duration_seconds"`
}

// handleOverride sets or clears manual sleep inhibition override.
func (s *Server) handleOverride(w http.ResponseWriter, r *http.Request) {
	var payload OverridePayload

	if r.Method == http.MethodPost {
		body, err := io.ReadAll(r.Body)
		if err == nil && len(body) > 0 {
			_ = json.Unmarshal(body, &payload)
		}
	} else if r.Method == http.MethodGet {
		q := r.URL.Query()
		payload.State = q.Get("state")
		payload.Duration = q.Get("duration")
		if durSec, err := strconv.Atoi(q.Get("duration_seconds")); err == nil {
			payload.DurationSeconds = durSec
		}
		if enStr := q.Get("enabled"); enStr != "" {
			en := strings.EqualFold(enStr, "true") || enStr == "1"
			payload.Enabled = &en
		}
	} else {
		sendJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}

	enabled := false
	if payload.Enabled != nil {
		enabled = *payload.Enabled
	} else {
		stLower := strings.ToLower(strings.TrimSpace(payload.State))
		enabled = stLower == "on" || stLower == "true" || stLower == "1" || stLower == "enable"
	}

	durStr := payload.Duration
	if durStr == "" && payload.DurationSeconds > 0 {
		durStr = fmt.Sprintf("%ds", payload.DurationSeconds)
	}

	st, err := s.engine.SetOverride(enabled, durStr)
	if err != nil {
		sendJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	sendJSON(w, http.StatusOK, st)
}

// handleCancel cancels active cooldown or manual override.
func (s *Server) handleCancel(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost && r.Method != http.MethodGet {
		sendJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}

	st, err := s.engine.Cancel()
	if err != nil {
		sendJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	sendJSON(w, http.StatusOK, st)
}

// handleHistory returns rolling transition journal entries.
func (s *Server) handleHistory(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		sendJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}

	limit := 50
	if limStr := r.URL.Query().Get("limit"); limStr != "" {
		if val, err := strconv.Atoi(limStr); err == nil && val > 0 {
			limit = val
		}
	}

	entries := s.engine.GetHistory(limit)
	sendJSON(w, http.StatusOK, entries)
}

// handleEvents establishes an SSE stream pushing state_change and heartbeat events.
func (s *Server) handleEvents(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	ch := s.engine.Subscribe()
	defer s.engine.Unsubscribe(ch)

	// Send initial state upon connection
	initialStatus := s.engine.GetStatus()
	initialData, _ := json.Marshal(initialStatus)
	fmt.Fprintf(w, "event: state_change\ndata: %s\n\n", string(initialData))
	flusher.Flush()

	ctx := r.Context()
	for {
		select {
		case <-ctx.Done():
			return
		case ev, open := <-ch:
			if !open {
				return
			}
			data, err := json.Marshal(ev.Status)
			if err == nil {
				fmt.Fprintf(w, "event: %s\ndata: %s\n\n", ev.Type, string(data))
				flusher.Flush()
			}
		}
	}
}
