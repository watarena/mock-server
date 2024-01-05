package main

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"
)

func (h *handler) String() string {
	resps := make([]response, len(h.responses))
	for i, r := range h.responses {
		resps[i] = *r
	}
	return fmt.Sprintf("%#v, responses: %#v", h, resps)
}

func (r *response) String() string {
	return fmt.Sprintf("%#v", *r)
}

// headerEqueal compares map[string][]string as header.
// Headers are equal if keys are the same ignoring case and
// their values are the same.
func headerEqueal(h1, h2 map[string][]string) bool {
	normalize := func(h map[string][]string) map[string][]string {
		m := make(map[string][]string)
		for k, v := range h {
			m[strings.ToLower(k)] = v
		}
		return m
	}

	return reflect.DeepEqual(normalize(h1), normalize(h2))
}

func TestNewServerSuccess(t *testing.T) {
	arg := &serverConfig{
		addr: ":1234",
		headers: []string{
			"header1: value1",
			"header2: value2-1",
			"header2: value2-2",
		},
		responses: []*responseConfig{
			{
				statusCode: 200,
				body:       []byte("OK"),
				headers: []string{
					"header3: value3",
				},
			},
			{
				statusCode: 400,
				body:       []byte("Bad Request"),
				headers: []string{
					"header2: respvalue2",
					"header3: value3",
				},
			},
		},
	}

	expectAddr := ":1234"
	expectHandler := &handler{
		responses: []*response{
			{
				statusCode: 200,
				body:       []byte("OK"),
				header: map[string][]string{
					"header1": {"value1"},
					"header2": {"value2-1", "value2-2"},
					"header3": {"value3"},
				},
			},
			{
				statusCode: 400,
				body:       []byte("Bad Request"),
				header: map[string][]string{
					"header1": {"value1"},
					"header2": {"respvalue2"},
					"header3": {"value3"},
				},
			},
		},
	}

	s, err := newServer(arg)
	if err != nil {
		t.Fatalf("error was not expected, but returned: %#v", err)
	}
	if s.Addr != expectAddr {
		t.Errorf("addr: expect %s but got %s", expectAddr, s.Addr)
	}
	actualHandler, ok := s.Handler.(*handler)
	if !ok {
		t.Fatal("Handler of server is not *hander type")
	}

	if actualHandler.shutdownServer == nil {
		t.Error("handler.shutdownServer should not be nil")
	}

	// check responses
	expectResps := expectHandler.responses
	actualResps := actualHandler.responses
	if len(expectResps) != len(actualResps) {
		t.Fatalf("responses do not match: expected %v, got: %v", expectResps, actualResps)
	}
	for i, expectRes := range expectResps {
		actualRes := actualResps[i]

		// check header
		if !headerEqueal(actualRes.header, expectRes.header) {
			t.Fatalf("header of %d-th responses do not match: expected %#v, got: %#v", i, expectRes, actualRes)
		}

		// check except header
		actualRes.header = nil
		expectRes.header = nil
		if !reflect.DeepEqual(actualRes, expectRes) {
			t.Fatalf("%d-th responses do not match: expected %#v, got: %#v", i, expectRes, actualRes)
		}
	}

	// check except responses and shutdownServer
	expectHandler.responses = nil
	actualHandler.responses = nil
	expectHandler.shutdownServer = nil
	actualHandler.shutdownServer = nil
	if !reflect.DeepEqual(actualHandler, expectHandler) {
		t.Errorf("handler: expect %v, but got %v", expectHandler, actualHandler)
	}
}

func TestNewServerFailure(t *testing.T) {
	cases := []struct {
		name string
		arg  *serverConfig
	}{
		{
			name: "invalidGrobalHeaderOption",
			arg: &serverConfig{
				headers: []string{"invalid header"},
			},
		},
		{
			name: "invalidResponseHeaderOption",
			arg: &serverConfig{
				responses: []*responseConfig{
					{
						headers: []string{"invalid header"},
					},
				},
			},
		},
	}

	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			server, err := newServer(c.arg)
			if err == nil {
				t.Error("error is expected, but got nil")
			}
			if server != nil {
				t.Errorf("server is expected to be nil, but not: %#v", server)
			}
		})
	}
}

func TestHandler_ServeHTTP(t *testing.T) {
	shutdownCh := make(chan struct{})
	handler := &handler{
		responses: []*response{
			{
				statusCode: 200,
				body:       []byte("OK"),
				header: map[string][]string{
					"header1": {"value1"},
				},
			},
			{
				statusCode: 400,
				body:       []byte("Bad Request"),
				header: map[string][]string{
					"header2": {"value2"},
				},
			},
		},
		shutdownServer: func() {
			close(shutdownCh)
		},
	}

	expectResps := []struct {
		code int
		body []byte
	}{
		{
			code: 200,
			body: []byte("OK"),
		},
		{
			code: 400,
			body: []byte("Bad Request"),
		},
	}

	for i, expect := range expectResps {
		if handler.pos != i {
			t.Errorf("handler.pos is expected to be %d, but %d", i, handler.pos)
		}
		w := httptest.NewRecorder()
		r := httptest.NewRequest("GET", "/", nil)

		handler.ServeHTTP(w, r)

		if w.Code != expect.code {
			t.Errorf("code does not match: expect %d, got: %d", expect.code, w.Code)
		}
		body := w.Body.Bytes()
		if !bytes.Equal(body, expect.body) {
			t.Errorf("body does not match: expect %s, got: %s", expect.body, body)
		}

		if handler.pos != i+1 {
			t.Errorf("handler.pos is expected to be %d, but %d", i+1, handler.pos)
		}
	}

	select {
	case <-shutdownCh:
	case <-time.After(time.Second):
		t.Fatal("shutdownServer was not called")

	}

	// test that ServeHTTP does not return
	c := make(chan struct{})
	w := httptest.NewRecorder()

	go func() {
		defer func() {
			recover()
			close(c)
		}()
		r := httptest.NewRequest("GET", "/", nil)
		handler.ServeHTTP(w, r)
	}()

	select {
	case <-c:
	case <-time.After(time.Second):
		t.Fatalf("Unexpected resonse was returned: code: %d, body: %s", w.Code, w.Body.String())

	}
	if w.Flushed {
		t.Errorf("response returned: code: %d, body: %s", w.Code, w.Body.String())
	}
}

func TestServer(t *testing.T) {
	l := httptest.NewUnstartedServer(nil).Listener

	requests := []struct {
		req        func(url string) *http.Request
		expectResp *response
	}{
		{
			req: func(url string) *http.Request {
				req, err := http.NewRequest("GET", url, nil)
				if err != nil {
					t.Fatalf("NewRequest failed %#v", err)
				}
				return req
			},
			expectResp: &response{
				statusCode: 200,
				body:       []byte("OK"),
			},
		},
		{
			req: func(url string) *http.Request {
				req, err := http.NewRequest("GET", url, nil)
				if err != nil {
					t.Fatalf("NewRequest failed %#v", err)
					return nil
				}
				req.Header.Add("Connection", "close")
				return req
			},
			expectResp: &response{
				statusCode: 400,
				body:       []byte("Bad Request"),
			},
		},
		{
			req: func(url string) *http.Request {
				req, err := http.NewRequest("GET", url, nil)
				if err != nil {
					t.Fatalf("NewRequest failed %#v", err)
				}
				return req
			},
			expectResp: &response{
				statusCode: 500,
				body:       []byte("Internal Server Error"),
			},
		},
	}

	server, err := newServer(&serverConfig{
		addr: ":0",
		headers: []string{
			"header1: value1",
			"header2: value2-1",
			"header2: value2-2",
		},
		responses: []*responseConfig{
			{
				statusCode: 200,
				body:       []byte("OK"),
				headers: []string{
					"header3: value3",
				},
			},
			{
				statusCode: 400,
				body:       []byte("Bad Request"),
				headers: []string{
					"header2: respvalue2",
					"header3: value3",
				},
			},
			{
				statusCode: 500,
				body:       []byte("Internal Server Error"),
				headers: []string{
					"header2: respvalue2",
					"header3: value3",
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("newServer failed: %#v", err)
	}
	c := make(chan error)
	go func() {
		c <- server.Serve(l)
	}()

	addr := "http://" + l.Addr().String()

	for _, r := range requests {
		req := r.req(addr)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("http.Get failed: %s", err)
		}
		defer resp.Body.Close()
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			t.Fatalf("http.Getreading body failed: %s", err)
		}

		if r.expectResp.statusCode != resp.StatusCode {
			t.Errorf("status code does not match: expected: %d, actual: %d", r.expectResp.statusCode, resp.StatusCode)
		}
		if !bytes.Equal(r.expectResp.body, body) {
			t.Errorf("body does not match: expected: %s, actual: %s", r.expectResp.body, string(body))
		}
	}

	select {
	case <-c:
	case <-time.After(time.Second):
		t.Error("server is not closed")
	}
}
