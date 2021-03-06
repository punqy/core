package core

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"reflect"

	logger "github.com/sirupsen/logrus"
	"github.com/valyala/fasthttp"
)

const (
	Get     = "GET"
	Head    = "HEAD"
	Post    = "POST"
	Put     = "PUT"
	Patch   = "PATCH"
	Delete  = "DELETE"
	Connect = "CONNECT"
	Options = "OPTIONS"
	Trace   = "TRACE"
)

type Handler func(Request) Response

type Middleware func(req Request, next Handler) Response

type MiddlewareChain []Middleware

type Request struct {
	*fasthttp.RequestCtx
}

func NewRequest(requestCtx *fasthttp.RequestCtx, route Route) Request {
	requestCtx.SetUserValue(RequestValueRoute, route)
	return Request{RequestCtx: requestCtx}
}

type Response interface {
	GetBytes() ([]byte, error)
	GetError() error
	GetCode() int
	GetHeaders() Headers
}

type Header struct {
	Name  string
	Value string
}

type Headers []Header

func (h Headers) Each(f func(name, val string)) {
	for _, h := range h {
		f(h.Name, h.Value)
	}
}

type RouteList []Route

type Attr map[string]interface{}

func (a Attr) Has(key string) bool {
	_, ok := a[key]
	return ok
}

func (a Attr) Get(key string) interface{} {
	return a[key]
}

type Route struct {
	Path    string
	Method  string
	Handler Handler
	Inner   RouteList
	Attr    Attr
}

func (r Request) ParseForm(dest interface{}) error {
	if reflect.TypeOf(dest).Kind() != reflect.Ptr {
		return errors.New("destination must be of type pointer")
	}
	if err := json.Unmarshal(r.PostBody(), dest); err != nil {
		return BadRequestErr("Invalid json schema")
	}
	return nil
}

func (r Request) Get(key string, def string) string {
	if r.URI().QueryArgs().Has(key) {
		return string(r.URI().QueryArgs().Peek(key))
	}
	return def
}

type Server interface {
	Serve(ctx context.Context)
}

type server struct {
	router     Router
	serverPort int
}

func NewHttpServer(router Router, serverPort int) Server {
	e := server{
		router:     router,
		serverPort: serverPort,
	}
	return &e
}

func (s *server) Serve(ctx context.Context) {
	logger.Infof("Http server listening port :%d", s.serverPort)
	server := &fasthttp.Server{Handler: s.router.GetMux().Handler}
	interrupt := make(chan os.Signal, 1)
	go func() {
		if err := server.ListenAndServe(fmt.Sprintf(":%d", s.serverPort)); err != nil {
			logger.Errorf("Http server down: %s", err)
			interrupt <- os.Interrupt
			return
		}
	}()
	signal.Notify(interrupt, os.Interrupt)
	<-interrupt
	s.shutdown(ctx, server)
}

func (s *server) shutdown(ctx context.Context, server *fasthttp.Server) {
	logger.Info("Sig interrupt received, graceful shutdown")
	if err := server.Shutdown(); err != nil {
		logger.Error("HttpServer shutdown err", err)
	}
	ctx.Done()
}
