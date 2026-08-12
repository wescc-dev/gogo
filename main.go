package main

import (
	"context"
	"gogopher/configuration"
	"gogopher/middleware"
	"gogopher/server"
	"gogopher/utility"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"
)

var cfg = configuration.GetConfiguration()

func main() {
	printConfiguration()
	svr := startServer()
	<-waitForShutdownSignal()
	log.Println("Shutdown signal received")
	_ = stopServer(svr)
	log.Println("Exiting normally.")

}

func printConfiguration() {
	log.Println("---------- GoGopher Server --------")
	log.Println(configuration.AppName + "(" + configuration.Version + ")")
	log.Println(configuration.Copyright)
	log.Println(configuration.Link)
	log.Println("---------- Configuration ----------")
	log.Println("Title:", cfg.Title)
	log.Println("Host Name:", cfg.HostName)
	log.Println("Host bind ip:", cfg.BindAddress)
	log.Println("Port:", cfg.Port)
	log.Println("Gopher root:", cfg.GopherRoot)
	log.Println("Firewall config:", cfg.FireWallConfigFile)
	log.Println("Idle timeout:", cfg.IdleTimeout)
	log.Println("Read/write timeout:", cfg.ReadWriteTimeout)
	log.Println("----------------------------------")

}

func startServer() server.IServer {
	log.Println("Starting server...")
	var svr, err = server.NewServer(cfg.HostName, cfg.BindAddress, cfg.Port, cfg.GopherRoot,
		cfg.IdleTimeout,
		cfg.ReadWriteTimeout)
	if svr != nil {
		middleware.AddFirewallMiddleware(svr)
	}
	if err != nil {
		log.Fatal(err)
	}
	if err := svr.Start(); err != nil {
		log.Fatal(err)
	}

	return svr
}

func stopServer(svr server.IServer) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var err = svr.Stop(ctx)
	if err != nil {
		log.Println(err)
	}

	log.Println("Shutdown complete.")
	log.Println("Server up time:", utility.FormatDuration(svr.UpTime()))
	return err
}

func waitForShutdownSignal() chan os.Signal {
	// Capture Ctrl‑C and SIGTERM
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	return sig
}
