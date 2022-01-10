package core

type ModuleProfiler interface {
	HttpProfilerMiddleware() HttpProfilerMiddleware
	ProfilerManager() ProfilerManager
}

type moduleProfiler struct {
	profilerManager        ProfilerManager
	httpProfilerMiddleware HttpProfilerMiddleware
}

func (m *moduleProfiler) HttpProfilerMiddleware() HttpProfilerMiddleware {
	return m.httpProfilerMiddleware
}

func (m *moduleProfiler) ProfilerManager() ProfilerManager {
	return m.profilerManager
}

func NewModuleProfiler(profilerEnabled bool, profileDir string) ModuleProfiler {
	var m moduleProfiler
	m.profilerManager = NewManager(profileDir)
	m.httpProfilerMiddleware = NewProfilerMiddleware(profilerEnabled, m.profilerManager)
	return &m
}
