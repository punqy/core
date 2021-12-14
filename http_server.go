package core

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/julienschmidt/httprouter"
	logger "github.com/sirupsen/logrus"
	"net"
	nethttp "net/http"
	"os"
	"os/signal"
	"reflect"
)

const (
	GET     = "GET"
	HEAD    = "HEAD"
	POST    = "POST"
	PUT     = "PUT"
	PATCH   = "PATCH"
	DELETE  = "DELETE"
	CONNECT = "CONNECT"
	OPTIONS = "OPTIONS"
	TRACE   = "TRACE"
)

type Handler func(Request) Response

type Middleware func(req Request, next Handler) Response
type MiddlewareChain []Middleware

type Request struct {
	*nethttp.Request
	Route  Route
	Params httprouter.Params
}

type Response interface {
	GetBytes() ([]byte, error)
	GetError() error
	GetCode() int
	GetHeaders() []Header
}

type Header struct {
	Name  string
	Value string
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
	decoder := json.NewDecoder(r.Body)
	err := decoder.Decode(dest)
	if err != nil {
		return BadRequestErr("Invalid json schema")
	}
	return nil
}

func (r Request) Get(key string, def string) string {
	if val := r.URL.Query().Get(key); val != "" {
		return val
	}
	return def
}


type Server interface {
	Serve(ctx context.Context)
}

type server struct {
	router         Router
	serverPort     int
}

func NewHttpServer(router Router, serverPort int) Server {
	e := server{
		router:         router,
		serverPort:     serverPort,
	}
	return &e
}

func (s *server) Serve(ctx context.Context) {
	logger.Infof("Http server listening port :%d", s.serverPort)
	server := &nethttp.Server{Addr: fmt.Sprintf(":%d", s.serverPort), Handler: s.router.GetMux()}
	interrupt := make(chan os.Signal, 1)
	go func() {
		if err := server.ListenAndServe(); err != nil {
			if nethttp.ErrServerClosed == err {
				logger.Error("Http server closed \u263E")
				return
			}
			if ne, ok := err.(*net.OpError); ok {
				logger.Error(ne)
				interrupt <- os.Interrupt
				return
			}
			logger.Error(err)
			return
		}
	}()
	signal.Notify(interrupt, os.Interrupt)

	<-interrupt
	s.shutdown(ctx, server)
}

func (s *server) shutdown(ctx context.Context, server *nethttp.Server) {
	logger.Info("Sig interrupt received, graceful shutdown")
	if err := server.Shutdown(ctx); err != nil {
		logger.Error("HttpServer shutdown err", err)
	}
	ctx.Done()
}

