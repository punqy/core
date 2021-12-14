package core

type ModuleProfiler interface {
	HttpProfilerMiddleware() HttpProfilerMiddleware
	ProfilerManager() Manager
}

type moduleProfiler struct {
	profilerManager        Manager
	httpProfilerMiddleware HttpProfilerMiddleware
}

func (m *moduleProfiler) HttpProfilerMiddleware() HttpProfilerMiddleware {
	return m.httpProfilerMiddleware
}

func (m *moduleProfiler) ProfilerManager() Manager {
	return m.profilerManager
}

func NewModuleProfiler(profilerEnabled bool, profileDir string) ModuleProfiler {
	var m moduleProfiler
	m.profilerManager = NewManager(profileDir)
	m.httpProfilerMiddleware = NewProfilerMiddleware(profilerEnabled, m.profilerManager)
	return &m
}
