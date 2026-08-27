package middleware

import (
	"log"
	"net"

	"github.com/wescc-dev/gogo/src/configuration"
	"github.com/wescc-dev/gogo/src/core"
	"github.com/wescc-dev/gogo/src/security"
)

var cfg = configuration.GetConfiguration()
var fw, firewallError = security.NewFireWall(cfg.FireWallConfigFile)

func AddFirewallMiddleware(svr core.Server) {
	if firewallError != nil {
		log.Fatal("Error loading firewall configuration: ", firewallError)
	}
	svr.AddMiddleware(firewallMiddleware)
}

var firewallMiddleware core.Middleware = func(next core.HandlerFunc) core.HandlerFunc {
	return func(
		ctx *core.RequestContext,
	) error {
		core.ContextLog(ctx, "Applying firewall rules")
		host, _, _ := net.SplitHostPort(ctx.Request.Conn.RemoteAddr().String())
		if err := fw.FirewallFilter(host); err != nil {
			core.ContextLog(ctx, "CLIENT BLOCKED BY FIREWALL:"+host)
			return err
		}
		core.ContextLog(ctx, "CLIENT ALLOWED BY FIREWALL:", host)

		return next(ctx)
	}
}
