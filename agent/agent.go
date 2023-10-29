package main

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/http/httputil"
	"os"
	"os/signal"
	"syscall"

	"github.com/gorilla/websocket"
	"github.com/hashicorp/yamux"
)

func Setup(connectURL string) error {
	dialer := websocket.DefaultDialer
	dialer.TLSClientConfig = &tls.Config{
		InsecureSkipVerify: true,
	}
	wsConn, _, err := dialer.Dial(connectURL, nil)
	if err != nil {
		return errors.New(fmt.Sprintf("failed to dial hub %q: %s", connectURL, err))
	}

	sess, err := yamux.Client(wsConn.UnderlyingConn(), nil)
	if err != nil {
		return errors.New(fmt.Sprintf("failed to create multiplex channel: %s", err))
	}
	log.Println("connected to hub")

	director := func(req *http.Request) {
		log.Println(req)
		host := req.Header.Get("X-Proxy-Host")
		path := req.Header.Get("X-Proxy-Path")
		req.Header.Add("X-Forwarded-Host", req.Host)
		req.Header.Add("X-Origin-Host", host)
		req.URL.Scheme = "http"
		req.URL.Host = host
		req.URL.Path = path
		req.Host = ""
		log.Println(req)
	}

	proxy := &httputil.ReverseProxy{Director: director}
	server := &http.Server{
		Handler: proxy,
	}

	idleConnsClosed := make(chan struct{})
	go func() {
		sigint := make(chan os.Signal, 1)
		signal.Notify(sigint, os.Interrupt)
		signal.Notify(sigint, syscall.SIGTERM)
		<-sigint

		if err := server.Shutdown(context.Background()); err != nil {
			log.Printf("failed to shutdown server: %s", err)
		}
		close(idleConnsClosed)
	}()

	log.Println("starting proxy")
	if err := server.Serve(sess); err != nil {
		return errors.New(fmt.Sprintf("error running proxy: %s", err))
	}

	<-idleConnsClosed

	return nil
}

func main() {
	Setup("121.37.9.9:8081")
}
