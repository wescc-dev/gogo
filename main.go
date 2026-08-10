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
	printConfiguration()
	svr, serverShutdown := startServer()
	defer func(svr server.IServer) {
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

func printConfiguration() {
	log.Println("----------Configuration ----------")
	log.Println(configuration.AppName + "(" + configuration.Version + ")")
	log.Println(configuration.Copyright)
	log.Println(configuration.Link)
	log.Println("Title:", cfg.Title)
	log.Println("Host:", cfg.Host)
	log.Println("Host bind ip:", cfg.HostBindIp)
	log.Println("Port:", cfg.Port)
	log.Println("Gopher root:", cfg.GopherRoot)
	log.Println("----------------------------------")

}

func startServer() (server.IServer, chan struct{}) {
	log.Println("Starting server...")
	serverShutdown := make(chan struct{})
	var svr, err = server.NewServer(cfg.Host, cfg.HostBindIp, cfg.Port, cfg.GopherRoot)
	if err != nil {
		log.Fatal(err)
	}
	go func() {
		if err := svr.ListenAndServe(); err != nil {
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

func stopServer(svr server.IServer) {
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
