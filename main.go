package main

import (
	"errors"
	"flag"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
)

var usageFormat = `Usage: %s [GROBAL OPTIONS] <status> <body> [RESPONSE OPTIONS] [<status> <body> [RESPONSE OPTIONS]]...
GROBAL OPTIONS:
  -c, --cert <cert file> Certificate file
  -H, --header <header> Add header to all responses
  -k, --key <key file> Private key file
  -p, --port <port> Port to listen (default: 8080)
RESPONSE OPTIONS:
  -H, --header <header> Add header to the response
  -r, --repeat <positive num> Repeat the response
`
var usage = fmt.Sprintf(usageFormat, filepath.Base(os.Args[0]))

func main() {
	sc, err := parseArgs(os.Args[1:])
	if err != nil {
		if errors.Is(err, flag.ErrHelp) {
			fmt.Print(usage)
			os.Exit(0)
		}
		fmt.Fprintln(os.Stderr, err)
		fmt.Fprint(os.Stderr, usage)
		os.Exit(1)
	}

	server, err := newServer(sc)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	if sc.tls != nil {
		err = server.ListenAndServeTLS(sc.tls.certFile, sc.tls.keyFile)
	} else {
		err = server.ListenAndServe()
	}

	if !errors.Is(err, http.ErrServerClosed) {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	<-server.shutdownCh
}
