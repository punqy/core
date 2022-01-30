package core

import (
	"bytes"
	"fmt"
	"html/template"
	"os"
	"path"
	"regexp"
	"strings"

	logger "github.com/sirupsen/logrus"
)

type block struct {
	name    string
	content string
}

type htmlTemplate struct {
	name   string
	raw    string
	parent *htmlTemplate
	blocks []block
}

type Vars map[string]interface{}

type TemplatingEngine interface {
	Render(tpl string, vars interface{}) (bytes.Buffer, error)
}

type engine struct {
	templateDir string
	templates   map[string]*template.Template

	functions   template.FuncMap
}

func NewTemplatingEngine(templateDir string, functions template.FuncMap) TemplatingEngine {
	e := &engine{
		templateDir: templateDir,
		templates:   make(map[string]*template.Template),
	}
	e.registerFunctions(functions)
	return e
}

func (e *engine) registerFunctions(functions template.FuncMap) {
	functions["include"] = func(tpl string, vars interface{}) template.HTML {
		buffer, err := e.Render(tpl, vars)
		if err != nil {
			logger.Error(err)
			return ""
		}
		return template.HTML(buffer.String())
	}
	e.functions = functions
}

func (e *engine) Render(tpl string, vars interface{}) (bytes.Buffer, error) {
	buf := bytes.Buffer{}
	t, err := parse(tpl, e.templateDir)
	cont := e.buildContent(t, []block{})
	tmpl, err := template.New(path.Base(tpl)).Funcs(e.functions).Parse(cont)
	if err != nil {
		return buf, err
	}
	err = tmpl.ExecuteTemplate(&buf, path.Base(tpl), vars)
	return buf, err
}

func (e *engine) buildContent(tpl htmlTemplate, blocks []block) string {
	if tpl.parent != nil {
		return e.buildContent(*tpl.parent, tpl.blocks)
	}
	for _, blk := range blocks {
		tpl.raw = regexp.MustCompile(fmt.Sprintf("{%%\\s*block\\s*(.[^<>]*?%s)\\s%%}(.*?){%%\\s*end\\s*%%}", blk.name)).ReplaceAllString(tpl.raw, blk.content)
	}
	return regexp.MustCompile("({%\\s*block\\s*(.*?)\\s*%}(.*?)({%\\s*end\\s*%}))").ReplaceAllString(tpl.raw, "")
}

func (e *engine) parseTemplate(fileName string, data interface{}) (bytes.Buffer, error) {
	buf := bytes.Buffer{}
	tmpl, err := e.getTemplate(fileName)
	if err != nil {
		return buf, err
	}
	err = tmpl.ExecuteTemplate(&buf, tmpl.Name(), data)
	if err != nil {
		return buf, err
	}
	return buf, nil
}

func (e *engine) getTemplate(name string) (*template.Template, error) {
	if tmpl, ok := e.templates[name]; ok {
		return tmpl, nil
	}
	templateLocation, err := e.PathTo(name)
	if err != nil {
		return nil, err
	}

	tmpl, err := template.New(path.Base(name)).Funcs(e.functions).ParseFiles(templateLocation)
	if err != nil {
		return nil, err
	}
	e.templates[name] = tmpl
	return tmpl, nil
}

func (e *engine) PathTo(name string) (string, error) {
	if !e.Exist(name) {
		return "", fmt.Errorf("file or directory (%s) does not exists", e.absolute(name))
	}
	return e.absolute(name), nil
}

func (e *engine) Exist(name string) bool {
	if _, err := os.Stat(e.absolute(name)); os.IsNotExist(err) {
		return false
	}
	return true
}

func (e *engine) absolute(name string) string {
	return normalize(fmt.Sprintf("/%s/%s", trim(e.templateDir), trim(name)))
}

func trim(part string) string {
	return strings.Trim(part, " ")
}

func normalize(path string) string {
	re := regexp.MustCompile(`/{2,}`)
	return re.ReplaceAllString(path, "/")
}

func parse(name string, rootDir string) (htmlTemplate, error) {
	tpl := htmlTemplate{
		name: name,
	}
	content, err := os.ReadFile(fmt.Sprintf("%s/%s", rootDir, tpl.name))
	tpl.raw = string(content)
	tpl.blocks = parseBlocks(tpl.raw)
	if parentName := parseParent(tpl.raw); parentName != "" {
		parentTpl, err := parse(parentName, rootDir)
		if err != nil {
			return tpl, err
		}
		tpl.parent = &parentTpl
	}
	return tpl, err
}

func parseParent(content string) string {
	matches := regexp.MustCompile("(?s){%\\s*extend\\s*(.*?[^\\s])\\s*%}").FindStringSubmatch(content)
	if matches == nil {
		return ""
	}
	return matches[1]
}

func parseBlocks(content string) []block {
	matches := regexp.MustCompile("(?s)({%\\s*block\\s*(.*?)\\s*%}(.*?)({%\\s*end\\s*%}))").FindAllStringSubmatch(content, -1)
	if matches == nil {
		return nil
	}
	blocks := make([]block, len(matches))
	for i, m := range matches {
		blocks[i] = block{
			name:    m[2],
			content: regexp.MustCompile("\\$").ReplaceAllString(m[3], "$$$"),
		}
	}
	return blocks
}
