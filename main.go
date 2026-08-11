package main

import (
	"context"
	"fmt"
	"gogopher/configuration"
	"gogopher/security"
	"gogopher/server"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"
)

var cfg = configuration.GetConfiguration()
var fw, firewallError = security.NewFireWall(cfg.FireWallConfigFile)

func main() {
	printConfiguration()
	if firewallError != nil {
		log.Fatal("Error loading firewall configuration: ", firewallError)
	}
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
	log.Println("Host:", cfg.Host)
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
	var svr, err = server.NewServer(cfg.Host, cfg.BindAddress, cfg.Port, cfg.GopherRoot,
		cfg.IdleTimeout,
		cfg.ReadWriteTimeout,
		useMiddleware())
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
	//<-svr.WaitForShutdown()

	log.Println("Shutdown complete.")
	log.Println("Server up time:", formatDuration(svr.UpTime()))
	return err
}

func useMiddleware() []server.Middleware {
	return []server.Middleware{
		func(next server.HandlerFunc) server.HandlerFunc {
			return func(conn net.Conn, rootDir string, selector string, timeout time.Duration) error {
				host, _, err := net.SplitHostPort(conn.RemoteAddr().String())
				if err != nil {
					log.Fatal("Error parsing remote address:", err)
					return err
				}
				log.Println("Request:", host, selector)
				if err := fw.FirewallFilter(host); err != nil {
					log.Println("CLIENT BLOCKED BY FIREWALL:", host)
					return err
				}
				r := next(conn, rootDir, selector, timeout)
				log.Println("Response:", conn.RemoteAddr(), selector)
				return r
			}
		}}
}

func waitForShutdownSignal() chan os.Signal {
	// Capture Ctrl‑C and SIGTERM
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	return sig
}

func formatDuration(d time.Duration) string {
	days := d / (24 * time.Hour)
	d -= days * 24 * time.Hour

	hours := d / time.Hour
	d -= hours * time.Hour

	minutes := d / time.Minute
	d -= minutes * time.Minute

	seconds := d / time.Second

	return fmt.Sprintf("%dd %dh %dm %ds", days, hours, minutes, seconds)
}
