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
	serverShutdown := make(chan struct{})
	log.Println("Starting server...")
	var svr = server.Server{
		Addr: cfg.HostBindIp + ":" + cfg.Port,
	}
	go func() {
		if err := svr.ListenAndServe(cfg.GopherRoot); err != nil {
			log.Fatal(err)
		}
		serverShutdown <- struct{}{}
	}()
	// Capture Ctrl‑C and SIGTERM
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig
	log.Println("Shutdown signal received")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var err = svr.Shutdown(ctx)
	if err != nil {
		log.Println(err)
	}
	log.Println("Shutdown complete")

	<-serverShutdown
	log.Println("Done.")
}
