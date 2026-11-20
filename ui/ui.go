// Package ui serves the settings window shared by the Windows and Linux apps:
// a small web page, embedded in the binary, that talks to the knob core over a
// loopback-only HTTP API. The page is shown in a browser app window. The
// package is platform-neutral and uses only the standard library; each
// platform supplies a Backend.
package ui

import (
	"crypto/rand"
	"crypto/subtle"
	"embed"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"net"
	"net/http"
	"strings"
	"sync/atomic"
	"time"
)

//go:embed web
var webFS embed.FS

// Backend is what a platform's knob core offers the settings page. Methods are
// called from HTTP goroutines; the backend does its own synchronization.
type Backend interface {
	// State is polled about twice a second and must be cheap.
	State() any
	// Config returns the active configuration as JSON.
	Config() ([]byte, error)
	// SaveConfig validates, writes and applies a configuration.
	SaveConfig(data []byte) error
	// Apps lists apps that can be given a profile (those running now).
	Apps() any
	// Command runs a named action such as "pause" or "reload".
	Command(name, arg string) error
	// Icon returns the window icon (.ico), or nil.
	Icon() []byte
}

// Server is the running settings server.
type Server struct {
	backend  Backend
	token    string
	host     string // "127.0.0.1:port", the only Host header accepted
	static   http.Handler
	lastSeen atomic.Int64
}

// Start listens on 127.0.0.1, on preferredPort when it is free so the page
// keeps one origin between runs, otherwise on any free port.
func Start(b Backend, preferredPort int) (*Server, error) {
	ln, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", preferredPort))
	if err != nil {
		if ln, err = net.Listen("tcp", "127.0.0.1:0"); err != nil {
			return nil, err
		}
	}
	raw := make([]byte, 24)
	if _, err := rand.Read(raw); err != nil {
		ln.Close()
		return nil, err
	}
	sub, err := fs.Sub(webFS, "web")
	if err != nil {
		ln.Close()
		return nil, err
	}
	s := &Server{backend: b, token: hex.EncodeToString(raw), host: ln.Addr().String(), static: http.FileServer(http.FS(sub))}
	srv := &http.Server{Handler: s, ReadHeaderTimeout: 5 * time.Second}
	go srv.Serve(ln)
	return s, nil
}

// URL is the address to open. The access token rides in the fragment, which is
// never sent to a server or written to a log.
func (s *Server) URL() string { return "http://" + s.host + "/#t=" + s.token }

// Active reports whether a settings page has talked to the server recently,
// i.e. a window is open.
func (s *Server) Active() bool {
	return time.Since(time.Unix(0, s.lastSeen.Load())) < 3*time.Second
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Refusing other Host values stops DNS-rebinding pages from reaching the API.
	if r.Host != s.host {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	h := w.Header()
	h.Set("Cache-Control", "no-store")
	h.Set("X-Content-Type-Options", "nosniff")
	h.Set("Content-Security-Policy", "default-src 'self'; style-src 'self' 'unsafe-inline'; frame-ancestors 'none'")
	switch {
	case strings.HasPrefix(r.URL.Path, "/api/"):
		s.api(w, r)
	case r.URL.Path == "/favicon.ico":
		icon := s.backend.Icon()
		if icon == nil {
			http.NotFound(w, r)
			return
		}
		h.Set("Content-Type", "image/x-icon")
		w.Write(icon)
	default:
		s.static.ServeHTTP(w, r)
	}
}

func (s *Server) api(w http.ResponseWriter, r *http.Request) {
	// The config can hold shell commands, so the API is closed to other pages:
	// a cross-site request cannot set the token header, and does not know it.
	if origin := r.Header.Get("Origin"); origin != "" && origin != "http://"+s.host {
		fail(w, http.StatusForbidden, "forbidden")
		return
	}
	if subtle.ConstantTimeCompare([]byte(r.Header.Get("X-Knob-Token")), []byte(s.token)) != 1 {
		fail(w, http.StatusUnauthorized, "stale or missing token")
		return
	}
	s.lastSeen.Store(time.Now().UnixNano())

	switch r.Method + " " + strings.TrimPrefix(r.URL.Path, "/api/") {
	case "GET state":
		reply(w, s.backend.State())
	case "GET apps":
		reply(w, s.backend.Apps())
	case "GET config":
		data, err := s.backend.Config()
		if err != nil {
			fail(w, http.StatusInternalServerError, err.Error())
			return
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.Write(data)
	case "POST config":
		data, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 1<<20))
		if err == nil {
			err = s.backend.SaveConfig(data)
		}
		if err != nil {
			fail(w, http.StatusBadRequest, err.Error())
			return
		}
		reply(w, map[string]bool{"ok": true})
	case "POST command":
		var c struct{ Name, Arg string }
		err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<16)).Decode(&c)
		if err == nil {
			err = s.backend.Command(c.Name, c.Arg)
		}
		if err != nil {
			fail(w, http.StatusBadRequest, err.Error())
			return
		}
		reply(w, map[string]bool{"ok": true})
	default:
		fail(w, http.StatusNotFound, "not found")
	}
}

func reply(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	json.NewEncoder(w).Encode(v)
}

func fail(w http.ResponseWriter, code int, message string) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(map[string]string{"error": message})
}
