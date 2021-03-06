package core

import (
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/fs"
	"io/ioutil"
	"os"
	"reflect"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/fatih/color"
	"github.com/sirupsen/logrus"
	logger "github.com/sirupsen/logrus"
)

const profileContextKey = "punqy-profile"

type qp []sqlQueryProfile

func (p qp) ByDateTime() []sqlQueryProfile {
	sort.SliceStable(p, func(i, j int) bool {
		return p[i].DateTime.UnixNano() < p[j].DateTime.UnixNano()
	})
	return p
}

func (p qp) ByHashSum() []sqlQueryProfile {
	sort.SliceStable(p, func(i, j int) bool {
		return p[i].Hash < p[j].Hash
	})
	return p
}

type Frame struct {
	Name string `json:"frame_name"`
	File string `json:"file"`
	Line string `json:"line"`
}

type sqlQueryProfile struct {
	Query    string        `json:"query"`
	Duration float64       `json:"duration"`
	Args     []interface{} `json:"args"`
	DateTime time.Time     `json:"date_time"`
	Hash     string        `json:"hash"`
}

func (q sqlQueryProfile) GetQuery() string {
	return q.Query
}

func (q sqlQueryProfile) GetDuration() float64 {
	return q.Duration
}

type SecurityContextProfile struct {
	Provider string `json:"provider"`
	Username string `json:"username"`
	UserID   string `json:"user_id"`
}

type EventProfile struct {
	EventName   string
	DateTime    time.Time
	Duration    float64
	Subscribers []string
}

type Profile struct {
	//Id id
	Id string `json:"id"`
	//DateTime http uri
	DateTime time.Time `json:"date_time"`
	//RequestURI http uri
	RequestURI string `json:"request_uri"`
	//RequestMethod http method
	RequestMethod string `json:"request_method"`
	//RequestMethod http method
	RequestBody string `json:"request_body"`
	//RequestHeaders http method
	RequestHeaders map[string]string `json:"request_headers"`
	//RequestHeaders http method
	ResponseHeaders map[string]string `json:"response_headers"`
	//ResponseCode http code
	ResponseCode int `json:"response_code"`
	//ResponseErr handle func
	ResponseErr string `json:"response_err"`
	//ErrTrace handle func
	ErrTrace []Frame `json:"err_trace"`
	//RemoteAddr ip
	RemoteAddr string `json:"remote_addr"`
	//RequestHandler handle func
	RequestHandler string `json:"request_handler"`
	//RequestDuration with time Duration
	RequestDuration float64 `json:"request_duration"`
	//MemoryUsed kilobytes
	MemoryUsed uint64 `json:"memory_used"`
	//SecurityContext with time Duration
	SecurityContext SecurityContextProfile `json:"security_context"`
	//SqlQueries with time Duration
	SqlQueries qp `json:"sql_queries"`
	//Events with time Duration
	Events []EventProfile `json:"events"`
}

func (l *Profile) SetSecurityContext(securityContext SecurityContext) {
	sc := SecurityContextProfile{
		Provider: securityContext.Token.Provider(),
	}
	user, ok := securityContext.Token.User().(UserInterface)
	if ok && user != nil {
		sc.Username = user.GetUsername()
		sc.UserID = user.GetID()
	}

	l.SecurityContext = sc
}

func (l *Profile) SetRequestDuration(duration float64) {
	l.RequestDuration = duration
}

func (l *Profile) Duration() float64 {
	return l.RequestDuration
}

func NewProfile(t time.Time) Profile {
	return Profile{
		Id:              fmt.Sprintf("%v", t.Unix()),
		DateTime:        t,
		SqlQueries:      make([]sqlQueryProfile, 0),
		RequestHeaders:  make(map[string]string),
		ResponseHeaders: make(map[string]string),
	}
}

func (l *Profile) GetId() string {
	return l.Id
}

func (l *Profile) QueryProfiles() []sqlQueryProfile {
	return l.SqlQueries
}

func (l *Profile) AddQueryProfile(query string, dur float64, args []interface{}) {
	qp := sqlQueryProfile{
		Query:    regexp.MustCompile("\\s{2,}").ReplaceAllString(query, " "),
		Duration: dur,
		Args:     args,
		DateTime: time.Now().UTC(),
	}
	hash := md5.Sum([]byte(qp.Query))
	qp.Hash = hex.EncodeToString(hash[:])
	l.SqlQueries = append(l.SqlQueries, qp)
}

func (l *Profile) AddEventDispatcherProfile(evt string, dur float64, subs EventSubscribers) {
	names := make([]string, len(subs))
	for i, s := range subs {
		names[i] = strings.Replace(runtime.FuncForPC(reflect.ValueOf(s).Pointer()).Name(), "-fm", "", 1)
	}
	ep := EventProfile{
		DateTime:    time.Now().UTC(),
		EventName:   evt,
		Duration:    dur,
		Subscribers: names,
	}
	l.Events = append(l.Events, ep)
}

func (l Profile) TotalQExecTime() float64 {
	var tet float64
	for _, ql := range l.SqlQueries {
		tet += ql.Duration
	}
	return tet
}

func (l Profile) TotalQExecTimeString() string {
	var tet float64
	for _, ql := range l.SqlQueries {
		tet += ql.Duration
	}
	return fmt.Sprintf("%.4f.s", l.TotalQExecTime())
}

func (l *Profile) PrintQueryLog() {
	for _, ql := range l.SqlQueries {
		logrus.Infof("%s [%.4f.s]", ql.Query, ql.Duration)
	}
}

type ProfilerManager interface {
	Save(Profile) error
	Last() (Profile, error)
	List() ([]Profile, error)
	Get(string) (Profile, error)
}

type profilerManager struct {
	profilerDir string
	profileDir  string
}

func NewManager(profilerDir string) ProfilerManager {
	profileDir := fmt.Sprintf("%s/profile", profilerDir)
	if err := Mkdir(profileDir); err != nil {
		panic(err)
	}
	return &profilerManager{
		profilerDir: profilerDir,
		profileDir:  profileDir,
	}
}

func (m *profilerManager) List() ([]Profile, error) {
	files, err := ioutil.ReadDir(m.profileDir)
	if err != nil {
		return []Profile{}, err
	}
	if len(files) > 60 {
		sort.SliceStable(files, func(i, j int) bool {
			return files[i].Name() > files[j].Name()
		})
		for i, file := range files {
			if i < 59 {
				continue
			}
			if err := os.Remove(fmt.Sprintf("%s/%s", m.profileDir, file.Name())); err != nil {
				return []Profile{}, err
			}
		}
		files, err = ioutil.ReadDir(m.profileDir)
		if err != nil {
			return []Profile{}, err
		}
	}
	var profiles = make([]Profile, len(files))
	sort.SliceStable(files, func(i, j int) bool {
		return files[i].Name() > files[j].Name()
	})
	total := len(files)
	if total == 0 {
		return profiles, nil
	}

	for i, file := range files {
		marshaled, err := os.ReadFile(fmt.Sprintf("%s/%s", m.profileDir, file.Name()))
		if err != nil {
			return profiles, nil
		}
		var profile Profile
		if err := json.Unmarshal(marshaled, &profile); err != nil {
			return profiles, err
		}
		profiles[i] = profile
	}
	return profiles, nil
}

func (m *profilerManager) Last() (Profile, error) {
	var p Profile
	files, err := ioutil.ReadDir(m.profileDir)
	if err != nil {
		return p, nil
	}
	sort.SliceStable(files, func(i, j int) bool {
		return files[i].Name() > files[j].Name()
	})
	if len(files) == 0 {
		return p, nil
	}
	marshaled, err := os.ReadFile(fmt.Sprintf("%s/%s", m.profileDir, files[0].Name()))
	if err != nil {
		return p, err
	}
	if err := json.Unmarshal(marshaled, &p); err != nil {
		return p, err
	}
	return p, nil
}

func (m *profilerManager) Get(id string) (Profile, error) {
	var p Profile
	marshaled, err := os.ReadFile(fmt.Sprintf("%s/%s.json", m.profileDir, id))
	if err != nil {
		return p, err
	}
	if err := json.Unmarshal(marshaled, &p); err != nil {
		return p, err
	}
	return p, nil
}

func (m *profilerManager) Save(profile Profile) error {
	if err := os.MkdirAll(m.profileDir, 0755); err != nil {
		return err
	}

	fileName := fmt.Sprintf("%s/%v.json", m.profileDir, profile.DateTime.Unix())
	file, err := os.Create(fileName)
	if err != nil {
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	marshaled, err := json.MarshalIndent(profile, "", "	")

	if err := ioutil.WriteFile(file.Name(), marshaled, fs.ModeDevice); err != nil {
		return err
	}
	return nil
}

type HttpProfilerMiddleware interface {
	Handle(req Request, next Handler) Response
}

type middleware struct {
	enabled bool
	manager ProfilerManager
	colors  colors
}

type colors struct {
	red    func(a ...interface{}) string
	yell   func(a ...interface{}) string
	blue   func(a ...interface{}) string
	cyan   func(a ...interface{}) string
	mage   func(a ...interface{}) string
	green  func(a ...interface{}) string
	cyanHi func(a ...interface{}) string
}

func NewProfilerMiddleware(enabled bool, manager ProfilerManager) HttpProfilerMiddleware {
	return &middleware{
		enabled: enabled,
		manager: manager,
		colors: colors{
			red:    color.New(color.FgRed).SprintFunc(),
			yell:   color.New(color.FgYellow).SprintFunc(),
			blue:   color.New(color.FgBlue).SprintFunc(),
			cyan:   color.New(color.FgCyan).SprintFunc(),
			cyanHi: color.New(color.FgHiCyan).SprintFunc(),
			mage:   color.New(color.FgHiMagenta).SprintFunc(),
			green:  color.New(color.FgHiGreen).SprintFunc(),
		},
	}
}

func (m *middleware) Handle(req Request, next Handler) Response {
	if !m.enabled {
		return next(req)
	}
	route, ok := req.UserValue(RequestValueRoute).(Route)
	if !ok {
		return next(req)
	}
	handler := strings.Replace(runtime.FuncForPC(reflect.ValueOf(route.Handler).Pointer()).Name(), "-fm", "", 1)
	if regexp.MustCompile("/*profilerHandler").MatchString(handler) {
		return next(req)
	}
	var msb runtime.MemStats
	runtime.ReadMemStats(&msb)

	profile := NewProfile(req.Time())
	req.RequestCtx.SetUserValue(profileContextKey, &profile)
	resp := next(req)

	var msa runtime.MemStats
	runtime.ReadMemStats(&msa)

	profile.RequestDuration = time.Now().Sub(req.Time()).Seconds()
	profile.MemoryUsed = (msa.TotalAlloc - msb.TotalAlloc) / 1024
	profile.RemoteAddr = req.RemoteAddr().String()
	profile.RequestMethod = string(req.Method())
	profile.RequestBody = string(req.PostBody())
	profile.ResponseCode = resp.GetCode()
	profile.RequestURI = req.URI().String()
	profile.RequestHandler = handler
	req.Request.Header.VisitAll(func(key, value []byte) {
		profile.RequestHeaders[string(key)] = string(value)
	})
	if err := resp.GetError(); err != nil {
		if stackErr, ok := err.(StackTracer); ok {
			profile.ErrTrace = make([]Frame, 0)
			for _, frame := range stackErr.StackTrace()[1:] {
				text, err := frame.MarshalText()
				if err != nil {
					return nil
				}
				frameVal := strings.Split(string(text), " ")
				fileLine := strings.Split(frameVal[1], ":")
				profile.ErrTrace = append(profile.ErrTrace, Frame{
					Name: frameVal[0],
					File: fileLine[0],
					Line: fileLine[1],
				})
			}
		}
		profile.ResponseErr = resp.GetError().Error()
	}

	if err := m.manager.Save(profile); err != nil {
		logger.Error(err)
		return resp
	}
	req.Response.Header.Add("x-request-profile-id", profile.Id)
	logger.WithFields(logger.Fields{
		profile.RequestMethod: profile.ResponseCode,
		"PID":                 profile.Id,
		"URI":                 profile.RequestURI,
		"IP":                  profile.RemoteAddr,
		"MEM":                 fmt.Sprintf("%d kib", profile.MemoryUsed),
		"DUR":                 fmt.Sprintf("%.4f.s", profile.RequestDuration),
	}).Infof(profile.RequestHandler)

	return resp
}
