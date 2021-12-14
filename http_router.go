package core

import (
	"fmt"
	"github.com/julienschmidt/httprouter"
	logger "github.com/sirupsen/logrus"
	nethttp "net/http"
	"strings"
)

type StaticFiles struct {
	Path    string
	RootDir nethttp.Dir
}

type RouterConfig struct {
	Routing         Route
	GlobalHandler   nethttp.Handler
	WSHandler       nethttp.Handler
	NotFoundHandler nethttp.Handler
	Middlewares     []Middleware
	StaticFiles     *StaticFiles
}

type Router interface {
	Apply(config Route, rtr nethttp.Handler, ancestorPattern string)
	GetMux() nethttp.Handler
}

type router struct {
	mux        nethttp.Handler
	routes     []Route
	middleware Middleware
}

func (r *router) GetMux() nethttp.Handler {
	return r.mux
}

func NewRouter(cfg RouterConfig) Router {
	mux := httprouter.New()
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
		mux.Handler(GET, "/ws", cfg.WSHandler)
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

func (r *router) Apply(config Route, rtr nethttp.Handler, ancestorPattern string) {
	router, ok := rtr.(*httprouter.Router)
	if !ok {
		panic("ERECW")
	}
	path := fmt.Sprintf("/%s/%s", strings.Trim(ancestorPattern, "/ "), strings.Trim(config.Path, "/ "))
	if len(config.Inner) > 0 {
		for _, nested := range config.Inner {
			r.Apply(nested, rtr, path)
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

func (r *router) createHandler(route Route) httprouter.Handle {
	return func(writer nethttp.ResponseWriter, request *nethttp.Request, params httprouter.Params) {
		defer func() {
			rec := recover()
			if rec != nil {
				writer.WriteHeader(nethttp.StatusInternalServerError)
				err := Wrap(fmt.Errorf("%v", rec))
				logger.Errorf("handler recovered from: %v", err)
			}
		}()
		req := r.createRequest(request, params, route)
		response := r.middleware(req, route.Handler)
		code := response.GetCode()
		if code == 0 {
			code = nethttp.StatusInternalServerError
		}
		for _, h := range response.GetHeaders() {
			writer.Header().Add(h.Name, h.Value)
		}
		writer.WriteHeader(code)
		bytes, err := response.GetBytes()
		if err != nil {
			response = NewResponse([]byte(""), fmt.Errorf("internal server error: %w", err), nethttp.StatusInternalServerError)
			bytes, _ = response.GetBytes()
		}
		_, err = writer.Write(bytes)
		if err != nil {
			logger.Error(err)
		}
	}
}

func (r *router) createRequest(request *nethttp.Request, params httprouter.Params, route Route) Request {
	return Request{Request: request, Params: params, Route: route}
}
