package core

import (
	"fmt"
	"strings"

	fasthttprouter "github.com/fasthttp/router"
	logger "github.com/sirupsen/logrus"
	"github.com/valyala/fasthttp"
)

type RequestValue string

const (
	RequestValueRoute = "route"
)

type MethodHandlerMap map[string]func(path string, handler fasthttp.RequestHandler)

type StaticFiles struct {
	Path    string
	RootDir string
}

type RouterConfig struct {
	Routing         Route
	GlobalHandler   fasthttp.RequestHandler
	WSHandler       fasthttp.RequestHandler
	NotFoundHandler fasthttp.RequestHandler
	Middlewares     []Middleware
	StaticFiles     *StaticFiles
}

type Router interface {
	Apply(config Route, router *fasthttprouter.Router, ancestorPattern string)
	GetMux() *fasthttprouter.Router
}

type router struct {
	mux        *fasthttprouter.Router
	routes     []Route
	middleware Middleware
}

func (r *router) GetMux() *fasthttprouter.Router {
	return r.mux
}

func NewRouter(cfg RouterConfig) Router {
	mux := fasthttprouter.New()
	if cfg.NotFoundHandler != nil {
		mux.NotFound = cfg.NotFoundHandler
	}
	if cfg.GlobalHandler != nil {
		mux.GlobalOPTIONS = cfg.GlobalHandler
	}
	if cfg.StaticFiles != nil {
		mux.ServeFiles(cfg.StaticFiles.Path, cfg.StaticFiles.RootDir)
	}
	if cfg.WSHandler != nil {
		mux.GET("/ws", cfg.WSHandler)
	}
	router := &router{mux: mux, middleware: chainMiddleware(cfg.Middlewares...)}
	router.Apply(cfg.Routing, mux, "")
	return router
}

func chainMiddleware(middlewares ...Middleware) Middleware {
	n := len(middlewares)
	return func(req Request, next Handler) Response {
		chainer := func(m Middleware, n Handler) Handler {
			return func(request Request) Response {
				return m(request, n)
			}
		}
		chainedHandler := next
		for i := n - 1; i >= 0; i-- {
			chainedHandler = chainer(middlewares[i], chainedHandler)
		}
		return chainedHandler(req)
	}
}

func (r *router) Apply(config Route, router *fasthttprouter.Router, ancestorPattern string) {
	path := fmt.Sprintf("/%s/%s", strings.Trim(ancestorPattern, "/ "), strings.Trim(config.Path, "/ "))
	if len(config.Inner) > 0 {
		for _, nested := range config.Inner {
			r.Apply(nested, router, path)
		}
		return
	}
	if config.Handler != nil {
		handler := r.createHandler(config)
		mm := MethodHandlerMap{
			Get:     router.GET,
			Post:    router.POST,
			Put:     router.PUT,
			Patch:   router.PATCH,
			Delete:  router.DELETE,
			Head:    router.HEAD,
			Options: router.POST,
			Trace:   router.TRACE,
		}
		if h, ok := mm[config.Method]; ok {
			h(path, handler)
		} else {
			router.ANY(path, handler)
		}
	}
}

func (r *router) createHandler(route Route) fasthttp.RequestHandler {
	return func(ctx *fasthttp.RequestCtx) {
		defer func() {
			rec := recover()
			if rec != nil {
				ctx.SetStatusCode(fasthttp.StatusInternalServerError)
				ctx.Response.SetBodyString("internal error")
				logger.Errorf("handler recovered from: %v", rec)
			}
		}()
		res := r.middleware(NewRequest(ctx, route), route.Handler)
		if ctx.Response.SetStatusCode(res.GetCode()); ctx.Response.StatusCode() == 0 {
			ctx.Response.SetStatusCode(fasthttp.StatusInternalServerError)
		}
		res.GetHeaders().Each(func(name, val string) {
			ctx.Response.Header.Add(name, val)
		})
		bytes, err := res.GetBytes()
		if err != nil {
			panic(err)
		}
		ctx.SetBody(bytes)
	}
}
