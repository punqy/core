package core

import (
	"fmt"
	fasthttprouter "github.com/fasthttp/router"
	logger "github.com/sirupsen/logrus"
	"github.com/valyala/fasthttp"
	nethttp "net/http"
	"strings"
)

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
		switch config.Method {
		case GET:
			router.GET(path, handler)
			break
		case POST:
			router.POST(path, handler)
			break
		case PUT:
			router.PUT(path, handler)
			break
		case PATCH:
			router.PATCH(path, handler)
			break
		case DELETE:
			router.DELETE(path, handler)
			break
		case HEAD:
			router.HEAD(path, handler)
			break
		case OPTIONS:
			router.OPTIONS(path, handler)
			break
		}
		return
	}
}

func (r *router) createHandler(route Route) fasthttp.RequestHandler {
	return func(ctx *fasthttp.RequestCtx) {
		defer func() {
			rec := recover()
			if rec != nil {
				ctx.SetStatusCode(nethttp.StatusInternalServerError)
				ctx.Response.SetBodyString("internal error")
				err := Wrap(fmt.Errorf("%v", rec))
				logger.Errorf("handler recovered from: %v", err)
			}
		}()
		req := r.createRequest(ctx, route)
		response := r.middleware(req, route.Handler)
		code := response.GetCode()
		if code == 0 {
			code = fasthttp.StatusInternalServerError
		}
		ctx.SetStatusCode(code)
		for _, h := range response.GetHeaders() {
			ctx.Response.Header.Add(h.Name, h.Value)
		}
		bytes, err := response.GetBytes()
		if err != nil {
			response = NewResponse([]byte(""), fmt.Errorf("internal server error: %w", err), nethttp.StatusInternalServerError)
			bytes, _ = response.GetBytes()
		}
		ctx.SetBody(bytes)
	}
}

func (r *router) createRequest(ctx *fasthttp.RequestCtx, route Route) Request {
	return Request{RequestCtx: ctx, Route: route}
}
