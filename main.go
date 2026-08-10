package main

import (
	"context"
	"gopher/configuration"
	"gopher/server"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"
)

var cfg = configuration.GetConfiguration()

func main() {
	svr, serverShutdown := startServer()
	defer func(svr *server.Server) {
		err := svr.Close()
		if err != nil {
			log.Println(err)
		}
	}(svr)
	<-waitForShutdownSignal()
	log.Println("Shutdown signal received")
	stopServer(svr)
	<-serverShutdown
	log.Println("Exiting normally.")

}

func startServer() (*server.Server, chan struct{}) {
	log.Println("Starting server...")
	serverShutdown := make(chan struct{})
	var svr = &server.Server{
		Addr: cfg.HostBindIp + ":" + cfg.Port,
	}
	go func() {
		if err := svr.ListenAndServe(cfg.GopherRoot); err != nil {
			log.Fatal(err)
		}
		close(serverShutdown)
	}()

	return svr, serverShutdown
}

func waitForShutdownSignal() chan os.Signal {
	// Capture Ctrl‑C and SIGTERM
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	return sig
}

func stopServer(svr *server.Server) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	log.Println("Shutting down server...")
	var err = svr.Shutdown(ctx)
	if err != nil {
		log.Println(err)
		return
	}
	log.Println("Shutdown complete.")

}
