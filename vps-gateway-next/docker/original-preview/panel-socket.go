package main

import (
	"errors"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"time"
)

const socketPath = "/panel/awg.sock"

func main() {
	if info, err := os.Lstat(socketPath); err == nil {
		if info.Mode()&os.ModeSocket == 0 {
			log.Fatal("panel path is not a Unix socket")
		}
		if err := os.Remove(socketPath); err != nil {
			log.Fatal(err)
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		log.Fatal(err)
	}

	listener, err := net.Listen("unix", socketPath)
	if err != nil {
		log.Fatal(err)
	}
	defer listener.Close()
	if err := os.Chmod(socketPath, 0600); err != nil {
		log.Fatal(err)
	}

	target := &url.URL{Scheme: "http", Host: "127.0.0.1:2222"}
	server := &http.Server{
		Handler:           httputil.NewSingleHostReverseProxy(target),
		ReadHeaderTimeout: 5 * time.Second,
	}
	if err := server.Serve(listener); !errors.Is(err, http.ErrServerClosed) {
		log.Print(err)
	}
}
