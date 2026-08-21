package core

import (
	"log"
)

func ContextLog(ctx *RequestContext, v ...any) {
	prefix := []any{"(", ctx.Request.RequestId, ")"}
	log.Println(append(prefix, v...)...)
}

func SystemLog(v ...any) {
	log.Println(append([]any{"GoGo:"}, v...)...)
}
