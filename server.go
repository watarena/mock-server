package main

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"log"
	"maps"
	"net/http"
	"net/http/httputil"
	"net/textproto"
	"os"
	"strings"
	"sync"
)

type serverConfig struct {
	addr      string
	headers   []string
	responses []*responseConfig
	tls       *tlsConfig
}

type responseConfig struct {
	statusCode int
	body       []byte
	headers    []string
}

type tlsConfig struct {
	certFile string
	keyFile  string
}

type response struct {
	statusCode int
	body       []byte
	header     map[string][]string
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
	if h.pos < len(h.responses) {
		h.pos++
		return h.responses[i], h.pos >= len(h.responses)
	}
	return nil, false
}

func (h *handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	resp, isLast := h.getResponse()
	if resp == nil {
		panic("all responses are used")
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

	for k, vs := range resp.header {
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

func parseHeaders(headerStrings []string) (map[string][]string, error) {
	bufr := bufio.NewReader(strings.NewReader(strings.Join(headerStrings, "\r\n") + "\r\n\r\n"))
	r := textproto.NewReader(bufr)
	header, err := r.ReadMIMEHeader()
	if err != nil {
		return nil, err
	}
	return header, nil
}

func newServer(c *serverConfig) (*server, error) {
	s := &http.Server{
		Addr:     c.addr,
		ErrorLog: log.New(io.Discard, "", 0),
	}

	handler, ch, err := newHandler(c.headers, c.responses, s)
	if err != nil {
		return nil, err
	}

	s.Handler = handler

	return &server{s, ch}, nil
}

func newHandler(grobalHeaderLines []string, respConfigs []*responseConfig, server *http.Server) (*handler, chan error, error) {
	ch := make(chan error)
	handler := &handler{
		shutdownServer: func() { ch <- server.Shutdown(context.Background()) },
	}

	grobalHeader, err := parseHeaders(grobalHeaderLines)
	if err != nil {
		return nil, nil, err
	}

	handler.responses = make([]*response, len(respConfigs))
	for i, rc := range respConfigs {
		r, err := newResponse(rc, grobalHeader)
		if err != nil {
			return nil, nil, err
		}
		handler.responses[i] = r
	}

	return handler, ch, nil
}

func newResponse(c *responseConfig, baseHeader map[string][]string) (*response, error) {
	r := &response{
		statusCode: c.statusCode,
		body:       c.body,
		header:     maps.Clone(baseHeader),
	}

	header, err := parseHeaders(c.headers)
	if err != nil {
		return nil, err
	}

	maps.Copy(r.header, header)

	return r, nil
}
