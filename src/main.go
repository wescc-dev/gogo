package main

import (
	"context"
	"gogopher/src/configuration"
	"gogopher/src/core"
	"gogopher/src/middleware"
	"gogopher/src/server"
	"gogopher/src/utility"
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
	log.Println("Request timeout (sec.):", cfg.RequestTimeoutDuration)
	log.Println("OS:", cfg.OS)
	log.Println("Architecture:", cfg.Architecture)
	log.Println("Number of CPUs:", cfg.NumCpus)
	log.Println("----------------------------------")

}

func startServer() core.IServer {
	log.Println("Starting server...")
	var svr, err = server.NewServer(
		cfg.Title,
		cfg.HostName,
		cfg.BindAddress,
		cfg.Port,
		cfg.GopherRoot,
		cfg.RequestTimeoutDuration,
		cfg.RequestMaximumBytes)
	if svr != nil {
		middleware.AddRequestId(svr)
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

func stopServer(svr core.IServer) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var err = svr.Stop(ctx)
	if err != nil {
		log.Println(err)
	}

	log.Println("Shutdown complete.")
	log.Println("Server up time:", utility.FormatDuration(svr.GetCurrentServerInfo().Uptime))
	return err
}

func waitForShutdownSignal() chan os.Signal {
	// Capture Ctrl‑C and SIGTERM
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	return sig
}
