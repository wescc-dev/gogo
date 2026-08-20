package middleware

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"gogo/src/core"
	"net"
	"time"
)

func AddRequestId(svr core.IServer) {
	svr.AddMiddleware(requestIdMiddleware)
}

func requestIdMiddleware(next core.HandlerFunc) core.HandlerFunc {
	return func(ctx *core.RequestContext) (err error) {
		requestID, err := newRequestID()
		if err != nil {
			return fmt.Errorf("cannot create request ID: %w", err)
		}
		ctx.Request.RequestId = requestID
		started := time.Now()
		host, _, err := net.SplitHostPort(ctx.Request.Conn.RemoteAddr().String())
		core.ContextLog(ctx, "Request started: ", host)
		defer func() {
			core.ContextLog(ctx, "Request completed:", "duration:", time.Since(started), "Error:", err)
		}()

		return next(ctx)
	}
}

func newRequestID() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
