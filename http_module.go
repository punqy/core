package core

type ModuleHttpServer interface {
	HttpServer() Server
	HttpRouter() Router
}

type module struct {
	server Server
	router Router
}

func NewHttpModule(listenPort int, routerConfig RouterConfig) ModuleHttpServer {
	var m module
	m.router = NewRouter(routerConfig)
	m.server = NewHttpServer(m.router, listenPort)
	return &m
}

func (m *module) HttpServer() Server {
	return m.server
}

func (m *module) HttpRouter() Router {
	return m.router
}
