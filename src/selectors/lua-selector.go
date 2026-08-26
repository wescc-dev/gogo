package selectors

import (
	"fmt"
	"gogo/src/core"
	"gogo/src/utility"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"

	"github.com/Shopify/go-lua"
	_ "github.com/Shopify/go-lua"
)

type LuaSelector struct {
	svrInfoProvider core.IServerInfoProvider
}

func NewLuaSelector(svrInfoProvider core.IServerInfoProvider) Selector {
	return &LuaSelector{svrInfoProvider: svrInfoProvider}
}

func (s *LuaSelector) Select(ctx *core.RequestContext) (*SelectResult, error) {
	scriptPath := filepath.Join(ctx.Request.RootDir, ctx.Request.Selector)
	if !utility.FileExists(scriptPath) {
		return &SelectResult{Handled: false}, nil
	}
	if !strings.HasSuffix(strings.ToLower(scriptPath), ".lua") {
		return &SelectResult{Handled: false}, nil
	}
	l := s.initLua(ctx)
	if err := lua.DoFile(l, scriptPath); err != nil {
		return nil, fmt.Errorf("execute Lua script: %w", err)
	}
	return &SelectResult{Handled: true}, nil
}

func (s *LuaSelector) initLua(ctx *core.RequestContext) *lua.State {
	l := lua.NewState()
	lua.OpenLibraries(l)
	_ = s.redirectLuaPrint(l, ctx.Request.Conn)
	s.pushStruct(l, s.svrInfoProvider.GetCurrentServerInfo())
	l.SetGlobal("context")
	s.registerFileReader(l)
	return l
}

func (s *LuaSelector) redirectLuaPrint(l *lua.State, w io.Writer) *error {
	var writeErr error

	l.Register("print", func(l *lua.State) int {
		n := l.Top()
		l.Global("tostring")

		for i := 1; i <= n; i++ {
			l.PushValue(-1) // tostring
			l.PushValue(i)
			l.Call(1, 1)

			value, ok := l.ToString(-1)
			if !ok {
				lua.Errorf(l, "'tostring' must return a string")
			}

			if writeErr == nil {
				if i > 1 {
					_, writeErr = io.WriteString(w, "\t")
				}
				if writeErr == nil {
					_, writeErr = io.WriteString(w, value)
				}
			}

			l.Pop(1)
		}

		if writeErr == nil {
			_, writeErr = io.WriteString(w, "\n")
		}

		return 0
	})

	return &writeErr
}

func (s *LuaSelector) registerFileReader(l *lua.State) {
	l.Register("read_file", func(l *lua.State) int {
		filename := lua.CheckString(l, 1)

		content, err := os.ReadFile(filename)
		if err != nil {
			l.PushNil()
			l.PushString(err.Error())
			return 2
		}

		l.PushString(string(content))
		return 1
	})
}

func (s *LuaSelector) pushStruct(l *lua.State, value any) {
	v := reflect.ValueOf(value)
	if v.Kind() == reflect.Pointer {
		v = v.Elem()
	}

	t := v.Type()
	l.CreateTable(0, v.NumField())

	for i := 0; i < v.NumField(); i++ {
		field := t.Field(i)
		value := v.Field(i)

		switch value.Kind() {
		case reflect.String:
			l.PushString(value.String())
		case reflect.Bool:
			l.PushBoolean(value.Bool())
		case reflect.Int, reflect.Int8, reflect.Int16,
			reflect.Int32, reflect.Int64:
			l.PushInteger(int(value.Int()))
		default:
			continue
		}

		l.SetField(-2, field.Name)
	}
}
