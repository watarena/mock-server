package main

import (
	"context"
	"fmt"
	"io"
	"maps"
	"net/http"
	"net/http/httputil"
	"os"
	"sync"
)

type serverConfig struct {
	addr      string
	headers   map[string][]string
	responses []*responseConfig
	tls       *tlsConfig
}

type responseConfig struct {
	statusCode int
	body       []byte
	headers    map[string][]string
}

type tlsConfig struct {
	certFile string
	keyFile  string
}

type response struct {
	statusCode int
	body       []byte
	headers    map[string][]string
}

type logger struct {
	mu sync.Mutex
}

func (l *logger) log(w io.Writer, msg string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	fmt.Fprintln(w, msg)
}

type handler struct {
	mu        sync.Mutex
	logger    logger
	responses []*response
	// shutdownServer shutdown the server of this handler
	shutdownServer func()
	// pos is the index of the next response.
	pos int
}

type server struct {
	*http.Server
	shutdownCh chan error
}

func (s *server) waitForShutDown() {
	<-s.shutdownCh
}

// getResponse returns the next response and wheather the response is the last if such a response exists,
// or nil, false if all responses were used.
func (h *handler) getResponse() (resp *response, isLast bool) {
	h.mu.Lock()
	defer h.mu.Unlock()
	i := h.pos
	if i < len(h.responses) {
		h.pos++
		return h.responses[i], h.pos >= len(h.responses)
	}
	return nil, false
}

func (h *handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	resp, isLast := h.getResponse()
	if resp == nil {
		panic(http.ErrAbortHandler)
	}

	if isLast {
		go h.shutdownServer()
	}

	reqBytes, err := httputil.DumpRequest(r, true)
	if err != nil {
		h.logger.log(os.Stderr, fmt.Sprintf("Failed to dump request: %v", err))
	} else {
		h.logger.log(os.Stdout, string(reqBytes))
	}

	for k, vs := range resp.headers {
		for i, v := range vs {
			if i == 0 {
				w.Header().Set(k, v)
			} else {
				w.Header().Add(k, v)
			}
		}
	}
	w.WriteHeader(resp.statusCode)
	w.Write(resp.body)
}

func newServer(c *serverConfig) *server {
	ch := make(chan error)
	s := &http.Server{
		Addr: c.addr,
	}

	handler := newHandler(c.headers, c.responses, func() { ch <- s.Shutdown(context.Background()) })

	s.Handler = handler

	return &server{s, ch}
}

func newHandler(grobalHeader map[string][]string, respConfigs []*responseConfig, shutdownFunc func()) *handler {
	handler := &handler{
		shutdownServer: shutdownFunc,
	}

	handler.responses = make([]*response, len(respConfigs))
	for i, rc := range respConfigs {
		r := newResponse(rc, grobalHeader)
		handler.responses[i] = r
	}

	return handler
}

func newResponse(c *responseConfig, baseHeader map[string][]string) *response {
	r := &response{
		statusCode: c.statusCode,
		body:       c.body,
		headers:    maps.Clone(baseHeader),
	}

	maps.Copy(r.headers, c.headers)

	return r
}
