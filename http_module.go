package core

type ModuleHttpServer interface {
	HttpServer() Server
	HttpRouter() Router
}

type moduleHttp struct {
	server Server
	router Router
}

func NewHttpModule(listenPort int, routerConfig RouterConfig) ModuleHttpServer {
	var m moduleHttp
	m.router = NewRouter(routerConfig)
	m.server = NewHttpServer(m.router, listenPort)
	return &m
}

func (m *moduleHttp) HttpServer() Server {
	return m.server
}

func (m *moduleHttp) HttpRouter() Router {
	return m.router
}
