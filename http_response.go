package core

import (
	"encoding/json"
	"github.com/pkg/errors"
	nethttp "net/http"
)

const (
	AcceptHeaderName             = "Accept"
	ContentTypeHeaderName        = "Content-type"
	ApplicationJsonHeaderVal     = "application/json"
	ApplicationTextHtmlHeaderVal = "text/html"
)

type response struct {
	bytes   []byte
	error   error
	code    int
	headers []Header
}

func NewResponse(bytes []byte, error error, code int, headers ...Header) Response {
	return &response{bytes: bytes, error: error, code: code, headers: headers}
}

func NewHtmlResponse(bytes []byte, code int) Response {
	return &response{bytes: bytes, code: code}
}

func NewErrorHtmlResponse(err error, code int) Response {
	return &response{bytes: []byte(err.Error()), error: err, code: code}
}

func NewRedirectResponse(location string) Response {
	return NewResponse(nil, nil, nethttp.StatusMovedPermanently, Header{
		Name:  "Location",
		Value: location,
	})
}

func (r response) GetBytes() ([]byte, error) {
	return r.bytes, nil
}

func (r *response) SetBytes(bytes []byte) {
	r.bytes = bytes
}

func (r response) GetError() error {
	return r.error
}

func (r *response) SetError(error error) {
	r.error = error
}

func (r response) GetCode() int {
	return r.code
}

func (r *response) SetCode(code int) {
	r.code = code
}

func (r response) GetHeaders() []Header {
	return r.headers
}

func (r *response) SetHeaders(headers []Header) {
	r.headers = headers
}

type jsonResponse struct {
	data    interface{}
	error   error
	code    int
	headers []Header
}

type JsonResponseFormat struct {
	Code    int         `json:"code"`
	Payload interface{} `json:"payload"`
}

func NewJsonResponse(data interface{}, code int, error error, headers ...Header) Response {
	return jsonResponse{data: data, code: code, error: error, headers: headers}
}

func (r jsonResponse) GetBytes() ([]byte, error) {
	marshaled, err := json.Marshal(JsonResponseFormat{
		Code:    r.code,
		Payload: r.data,
	})
	if err != nil {
		return nil, err
	}

	return marshaled, nil
}

func (r jsonResponse) GetError() error {
	return r.error
}

func (r *jsonResponse) SetError(error error) {
	r.error = error
}

func (r jsonResponse) GetCode() int {
	return r.code
}

func (r *jsonResponse) SetCode(code int) {
	r.code = code
}

func (r jsonResponse) GetHeaders() []Header {
	return append(r.headers, Header{ContentTypeHeaderName, ApplicationJsonHeaderVal})
}

func (r *jsonResponse) SetHeaders(headers []Header) {
	r.headers = headers
}

func NewErrorJsonResponse(error error, headers ...Header) Response {
	if error == nil {
		return NewJsonResponse(nil, nethttp.StatusOK, error, headers...)
	}
	nextCode := nethttp.StatusInternalServerError
	var er erro
	if errors.As(error, &er) {
		nextCode = er.GetCode()
	}
	return NewJsonResponse(error.Error(), nextCode, error, headers...)
}
