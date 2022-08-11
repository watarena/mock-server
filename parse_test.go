package main

import (
	"errors"
	"flag"
	"fmt"
	"reflect"
	"testing"
)

func serverToString(s *serverConfig) string {
	resps := make([]responseConfig, len(s.responses))
	for i, r := range s.responses {
		resps[i] = *r
	}
	return fmt.Sprintf("%#v, responses: %#v", *s, resps)
}

func TestParseArgsSuccess(t *testing.T) {
	cases := []struct {
		name   string
		args   []string
		expect *serverConfig
	}{
		{
			name: "WithoutGrobalOptions",
			args: []string{
				"200",
				"OK",
				"-r",
				"2",
				"--header",
				"test-header: header",
				"400",
				"Bad Request",
				"--repeat",
				"3",
				"-H",
				"test-headers: value1",
				"-H",
				"test-headers: value2",
			},
			expect: &serverConfig{
				addr:    ":8080",
				headers: []string{},
				responses: func() []*responseConfig {
					resp1 := &responseConfig{
						statusCode: 200,
						body:       "OK",
						headers: []string{
							"test-header: header",
						},
					}
					resp2 := &responseConfig{
						statusCode: 400,
						body:       "Bad Request",
						headers: []string{
							"test-headers: value1",
							"test-headers: value2",
						},
					}
					return []*responseConfig{resp1, resp1, resp2, resp2, resp2}
				}(),
			},
		},
		{
			name: "WithLongGrobalOptions",
			args: []string{
				"--port",
				"1234",
				"--header",
				"grobal-header: grobal1",
				"--header",
				"grobal-header: grobal2",
				"200",
				"OK",
				"-r",
				"2",
				"--header",
				"test-header: header",
				"400",
				"Bad Request",
				"--repeat",
				"3",
				"-H",
				"test-headers: value1",
				"-H",
				"test-headers: value2",
			},
			expect: &serverConfig{
				addr: ":1234",
				headers: []string{
					"grobal-header: grobal1",
					"grobal-header: grobal2",
				},
				responses: func() []*responseConfig {
					resp1 := &responseConfig{
						statusCode: 200,
						body:       "OK",
						headers: []string{
							"test-header: header",
						},
					}
					resp2 := &responseConfig{
						statusCode: 400,
						body:       "Bad Request",
						headers: []string{
							"test-headers: value1",
							"test-headers: value2",
						},
					}
					return []*responseConfig{resp1, resp1, resp2, resp2, resp2}
				}(),
			},
		},
		{
			name: "WithShortGrobalOptions",
			args: []string{
				"-p",
				"1234",
				"-H",
				"grobal-header: grobal1",
				"-H",
				"grobal-header: grobal2",
				"200",
				"OK",
				"-r",
				"2",
				"--header",
				"test-header: header",
				"400",
				"Bad Request",
				"--repeat",
				"3",
				"-H",
				"test-headers: value1",
				"-H",
				"test-headers: value2",
			},
			expect: &serverConfig{
				addr: ":1234",
				headers: []string{
					"grobal-header: grobal1",
					"grobal-header: grobal2",
				},
				responses: func() []*responseConfig {
					resp1 := &responseConfig{
						statusCode: 200,
						body:       "OK",
						headers: []string{
							"test-header: header",
						},
					}
					resp2 := &responseConfig{
						statusCode: 400,
						body:       "Bad Request",
						headers: []string{
							"test-headers: value1",
							"test-headers: value2",
						},
					}
					return []*responseConfig{resp1, resp1, resp2, resp2, resp2}
				}(),
			},
		},
	}

	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()

			actual, err := parseArgs(c.args)
			if err != nil {
				t.Fatalf("error was not expected but got: %#v", err)
			}
			if !reflect.DeepEqual(actual, c.expect) {
				t.Errorf("expect %s, but got %s", serverToString(actual), serverToString(c.expect))
			}
		})
	}
}

func TestParseArgsFailure(t *testing.T) {
	cases := []struct {
		name string
		args []string
	}{
		{
			name: "InvalidPort",
			args: []string{
				"-p",
				"port",
				"200",
				"OK",
			},
		},
		{
			name: "NoStatusCode",
			args: []string{},
		},
		{
			name: "NoBody",
			args: []string{
				"200",
			},
		},
		{
			name: "InvalidRepeat",
			args: []string{
				"200",
				"OK",
				"-r",
				"invalid",
			},
		},
		{
			name: "NegativeRepeat",
			args: []string{
				"200",
				"OK",
				"-r",
				"-1",
			},
		},
		{
			name: "ZeroRepeat",
			args: []string{
				"200",
				"OK",
				"-r",
				"0",
			},
		},
	}

	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()

			_, err := parseArgs(c.args)
			if err == nil {
				t.Fatalf("error was expected but no error returned")
			}
		})
	}
}

func TestParseArgsHelpOption(t *testing.T) {
	cases := []struct {
		name string
		args []string
	}{
		{
			name: "ShortInGrobalOption",
			args: []string{
				"--port",
				"1234",
				"--header",
				"grobal-header: grobal1",
				"-h",
				"--header",
				"grobal-header: grobal2",
				"200",
				"OK",
				"-r",
				"2",
				"--header",
				"test-header: header",
				"400",
				"Bad Request",
				"--repeat",
				"3",
				"-H",
				"test-headers: value1",
				"-H",
				"test-headers: value2",
			},
		},
		{
			name: "LongInGrobalOption",
			args: []string{
				"--port",
				"1234",
				"--header",
				"grobal-header: grobal1",
				"--help",
				"--header",
				"grobal-header: grobal2",
				"200",
				"OK",
				"-r",
				"2",
				"--header",
				"test-header: header",
				"400",
				"Bad Request",
				"--repeat",
				"3",
				"-H",
				"test-headers: value1",
				"-H",
				"test-headers: value2",
			},
		},
		{
			name: "ShortInResponseOption",
			args: []string{
				"--port",
				"1234",
				"--header",
				"grobal-header: grobal1",
				"--header",
				"grobal-header: grobal2",
				"200",
				"OK",
				"-r",
				"2",
				"--header",
				"test-header: header",
				"-h",
				"400",
				"Bad Request",
				"--repeat",
				"3",
				"-H",
				"test-headers: value1",
				"-H",
				"test-headers: value2",
			},
		},
		{
			name: "LongInResponseOption",
			args: []string{
				"--port",
				"1234",
				"--header",
				"grobal-header: grobal1",
				"--header",
				"grobal-header: grobal2",
				"200",
				"OK",
				"-r",
				"2",
				"--header",
				"test-header: header",
				"--help",
				"400",
				"Bad Request",
				"--repeat",
				"3",
				"-H",
				"test-headers: value1",
				"-H",
				"test-headers: value2",
			},
		},
	}

	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()

			_, err := parseArgs(c.args)
			if !errors.Is(err, flag.ErrHelp) {
				t.Errorf("flag.ErrHelp was expected but not: %#v", err)
			}
		})
	}
}
