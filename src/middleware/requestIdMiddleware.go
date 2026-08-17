package middleware

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"gogopher/src/core"
	"log"
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

		log.Println("Request started:", requestID)
		defer func() {
			log.Println("Request completed:", requestID, "duration:", time.Since(started), "error:", err)
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
