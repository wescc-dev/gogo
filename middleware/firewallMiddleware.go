package middleware

import (
	"gogopher/configuration"
	"gogopher/security"
	"gogopher/server"
	"log"
	"net"
	"time"
)

var cfg = configuration.GetConfiguration()
var fw, firewallError = security.NewFireWall(cfg.FireWallConfigFile)

func AddFirewallMiddleware(svr server.IServer) {
	if firewallError != nil {
		log.Fatal("Error loading firewall configuration: ", firewallError)
	}
	svr.AddMiddleware(firewallMiddleware)
}

var firewallMiddleware server.Middleware = func(next server.HandlerFunc) server.HandlerFunc {
	return func(
		conn net.Conn,
		rootDir string,
		selector string,
		timeout time.Duration,
	) error {
		log.Println("Request:", conn.RemoteAddr(), selector)
		log.Println("Applying firewall rules...")
		host, _, err := net.SplitHostPort(conn.RemoteAddr().String())
		if err != nil {
			return err
		}

		if err := fw.FirewallFilter(host); err != nil {
			log.Println("CLIENT BLOCKED BY FIREWALL:", host)
			return err
		}
		log.Println("CLIENT ALLOWED BY FIREWALL:", host)

		return next(conn, rootDir, selector, timeout)
	}
}
