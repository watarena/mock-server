package main

import (
	"bufio"
	"bytes"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/textproto"
	"os"
	"strconv"
	"strings"
)

const (
	defaultPort = 8080
)

// optArrayString is string array implementing flag.Value
type optStringArray []string

var _ flag.Value = (*optStringArray)(nil)

func (a *optStringArray) String() string {
	return fmt.Sprintf("%v", *a)
}

func (a *optStringArray) Set(s string) error {
	*a = append(*a, s)
	return nil
}

type loadBody func(string) ([]byte, error)

var (
	loadBodyRaw  loadBody = func(s string) ([]byte, error) { return []byte(s), nil }
	loadBodyFile loadBody = func(s string) ([]byte, error) { return os.ReadFile(s) }
)

func parseArgs(args []string) (*serverConfig, error) {
	server, rest, err := parseGrobalOptions(args)
	if err != nil {
		return nil, err
	}

	resps, err := parseResponsesPart(rest)
	if err != nil {
		return nil, err
	}
	server.responses = resps

	return server, nil
}

func parseGrobalOptions(args []string) (*serverConfig, []string, error) {
	f := flag.NewFlagSet("", flag.ContinueOnError)
	f.Usage = func() {}
	f.SetOutput(io.Discard)

	optPort := defaultPort
	optHeaders := optStringArray([]string{})
	optCertFile := ""
	optCertKeyFile := ""

	f.IntVar(&optPort, "p", defaultPort, "")
	f.IntVar(&optPort, "port", defaultPort, "")
	f.Var(&optHeaders, "H", "")
	f.Var(&optHeaders, "header", "")
	f.StringVar(&optCertFile, "c", "", "")
	f.StringVar(&optCertFile, "cert", "", "")
	f.StringVar(&optCertKeyFile, "k", "", "")
	f.StringVar(&optCertKeyFile, "key", "", "")

	if err := f.Parse(args); err != nil {
		return nil, nil, err
	}

	var tls *tlsConfig
	if optCertFile != "" && optCertKeyFile != "" {
		tls = &tlsConfig{
			certFile: optCertFile,
			keyFile:  optCertKeyFile,
		}
	} else if optCertFile != "" && optCertKeyFile == "" {
		return nil, nil, errors.New("key option is not set")
	} else if optCertFile == "" && optCertKeyFile != "" {
		return nil, nil, errors.New("cert option is not set")
	}

	headers, err := parseHeaders(optHeaders)
	if err != nil {
		return nil, nil, err
	}

	return &serverConfig{
		addr:    fmt.Sprintf(":%d", optPort),
		headers: headers,
		tls:     tls,
	}, f.Args(), nil
}

func repeatResponse(resp *responseConfig, repeat int) []*responseConfig {
	resps := make([]*responseConfig, repeat)
	for i := range resps {
		resps[i] = resp
	}
	return resps
}

// parseResponsesPart parses repeat of <status> <body> [options]...
func parseResponsesPart(args []string) ([]*responseConfig, error) {
	if len(args) < 2 {
		return nil, errors.New("status code and body are required")
	}

	resps := []*responseConfig{}

	rest := args
	for len(rest) > 0 {
		if len(rest) < 2 {
			return nil, errors.New("status code and body are required")
		}
		statusCode, err := strconv.Atoi(rest[0])
		if err != nil {
			return nil, err
		}
		bodyArg := rest[1]

		f := flag.NewFlagSet("", flag.ContinueOnError)
		f.Usage = func() {}
		f.SetOutput(io.Discard)

		repeat := 1
		optHeaders := optStringArray([]string{})
		loadBody := loadBodyRaw
		trimNewline := false

		f.IntVar(&repeat, "r", 1, "")
		f.IntVar(&repeat, "repeat", 1, "")
		f.Var(&optHeaders, "H", "")
		f.Var(&optHeaders, "header", "")
		f.BoolFunc("body-file", "", func(_ string) error { loadBody = loadBodyFile; return nil })
		f.BoolVar(&trimNewline, "trim-newline", false, "")

		if err := f.Parse(rest[2:]); err != nil {
			return nil, err
		}

		if repeat <= 0 {
			return nil, errors.New("repeat must be positive")
		}

		body, err := loadBody(bodyArg)
		if err != nil {
			return nil, err
		}

		if trimNewline {
			body = bytes.Trim(body, "\n")
		}

		headers, err := parseHeaders(optHeaders)
		if err != nil {
			return nil, err
		}

		resp := &responseConfig{
			statusCode: statusCode,
			body:       []byte(body),
			headers:    headers,
		}
		resps = append(resps, repeatResponse(resp, repeat)...)
		rest = f.Args()
	}

	return resps, nil
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
