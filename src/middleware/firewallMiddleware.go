package middleware

import (
	"gogopher/src/configuration"
	"gogopher/src/core"
	"gogopher/src/security"
	"log"
	"net"
	"time"
)

var cfg = configuration.GetConfiguration()
var fw, firewallError = security.NewFireWall(cfg.FireWallConfigFile)

func AddFirewallMiddleware(svr core.IServer) {
	if firewallError != nil {
		log.Fatal("Error loading firewall configuration: ", firewallError)
	}
	svr.AddMiddleware(firewallMiddleware)
}

var firewallMiddleware core.Middleware = func(next core.HandlerFunc) core.HandlerFunc {
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
