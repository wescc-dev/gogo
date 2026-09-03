package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/spf13/pflag"
	"github.com/wescc-dev/gogo/src/configuration"
	"github.com/wescc-dev/gogo/src/core"
	"github.com/wescc-dev/gogo/src/middleware"
	"github.com/wescc-dev/gogo/src/server"
	"github.com/wescc-dev/gogo/src/utility"
)

var cfg = configuration.GetConfiguration()

func main() {
	parseFlags()
	printConfiguration()
	svr := startServer()
	<-waitForShutdownSignal()
	log.Println("Shutdown signal received")
	_ = stopServer(svr)
	log.Println("Exiting normally.")

}

func startServer() core.Server {
	log.Println("Starting server...")
	var svr, err = server.NewGopherServer(
		cfg.Title,
		cfg.HostName,
		cfg.BindAddress,
		cfg.Port,
		cfg.GopherRoot,
		cfg.RequestTimeoutDuration,
		cfg.RequestMaximumBytes,
		cfg.TLSCertFile,
		cfg.TLSKeyFile)
	if err != nil {
		log.Fatal(err)
	}
	if svr != nil {
		middleware.AddRequestId(svr)
		middleware.AddFirewallMiddleware(svr)
	}
	if err := svr.ListenAndServe(); err != nil {
		log.Fatal(err)
	}
	return svr
}

func stopServer(svr core.Server) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var err = svr.Shutdown(ctx)
	if err != nil {
		log.Println(err)
	}

	core.SystemLog("Shutdown complete.")
	core.SystemLog("Server up time:", utility.FormatDuration(svr.GetCurrentServerInfo().Uptime))
	return err
}

func waitForShutdownSignal() chan os.Signal {
	// Capture Ctrl‑C and SIGTERM
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	return sig
}

func parseFlags() {
	help := pflag.BoolP("help", "h", false, "Show help")
	info := pflag.BoolP("info", "i", false, "Show server info")
	config := pflag.BoolP("config", "c", false, "Show server configuration")
	title := pflag.StringP("title", "t", cfg.Title, "Server title")
	hostName := pflag.StringP("hostname", "n", cfg.HostName, "Server hostname")
	hostBindAddress := pflag.StringP("bind", "b", cfg.BindAddress, "Server bind address")
	port := pflag.StringP("port", "p", cfg.Port, "Server port")
	gopherRoot := pflag.StringP("gopher-root", "r", cfg.GopherRoot, "Gopher root directory")
	firewallConfigFile := pflag.StringP("firewall-config", "f", cfg.FireWallConfigFile, "Firewall configuration file")
	fileAccessConfigFile := pflag.StringP("file-access-config", "a", cfg.FileAccessConfigFile, "File access configuration file")
	itemTypeConfigFile := pflag.StringP("item-type-config", "g", cfg.ItemTypeConfigFile, "Item type configuration file")
	requestTimeout := pflag.StringP("request-timeout", "o", cfg.RequestTimeoutDuration.String(), "Request timeout duration")
	requestMaximumBytes := pflag.IntP("request-maximum-bytes", "m", cfg.RequestMaximumBytes, "Request maximum bytes")
	pflag.CommandLine.ParseErrorsAllowlist = pflag.ParseErrorsAllowlist{
		UnknownFlags: true,
	}

	pflag.Parse()
	if *info {
		printInfo()
		os.Exit(0)
	}
	if *config {
		printConfiguration()
		os.Exit(0)
	}
	if *help {
		pflag.Usage()
		os.Exit(0)
	}
	cfg.Title = *title
	cfg.HostName = *hostName
	cfg.BindAddress = *hostBindAddress
	cfg.Port = *port
	cfg.GopherRoot = *gopherRoot
	cfg.FireWallConfigFile = *firewallConfigFile
	cfg.FileAccessConfigFile = *fileAccessConfigFile
	cfg.ItemTypeConfigFile = *itemTypeConfigFile
	cfg.RequestTimeoutDuration, _ = time.ParseDuration(*requestTimeout)
	cfg.RequestMaximumBytes = *requestMaximumBytes
}

func printConfiguration() {
	printInfo()
	log.Println("------------ Configuration -----------")
	log.Println("Title:", cfg.Title)
	log.Println("Host Name:", cfg.HostName)
	log.Println("Host bind ip:", cfg.BindAddress)
	log.Println("Port:", cfg.Port)
	log.Println("TLS certificate:", cfg.TLSCertFile)
	log.Println("TLS key:", cfg.TLSKeyFile)
	log.Println("Gopher root:", cfg.GopherRoot)
	log.Println("Firewall config:", cfg.FireWallConfigFile)
	log.Println("Request timeout (sec.):", cfg.RequestTimeoutDuration)
	log.Println("OS:", cfg.OS)
	log.Println("Architecture:", cfg.Architecture)
	log.Println("Number of CPUs:", cfg.CoreCount)
	log.Println("-------------------------------------")
}

func printInfo() {
	log.Println("---------- GoGo Gopher Server --------")
	log.Println(cfg.Metadata.AppName, "("+cfg.Metadata.Version+")")
	log.Println(cfg.Metadata.Copyright, "("+cfg.Metadata.License+")")
	log.Println(cfg.Metadata.Link)
}
