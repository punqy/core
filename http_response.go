package core

import (
	"database/sql"
	"encoding/json"

	validation "github.com/go-ozzo/ozzo-validation/v4"
	"github.com/lib/pq"
	"github.com/pkg/errors"
	"github.com/valyala/fasthttp"
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
	headers Headers
}

func NewResponse(bytes []byte, error error, code int, headers ...Header) Response {
	return &response{bytes: bytes, error: error, code: code, headers: headers}
}

func NewHtmlResponse(bytes []byte, code int) Response {
	return &response{bytes: bytes, code: code, headers: Headers{
		{
			Name:  ContentTypeHeaderName,
			Value: ApplicationTextHtmlHeaderVal,
		},
	}}
}

func NewErrorHtmlResponse(err error, code int) Response {
	return &response{bytes: []byte(err.Error()), error: err, code: code, headers: Headers{
		{
			Name:  ContentTypeHeaderName,
			Value: ApplicationTextHtmlHeaderVal,
		},
	}}
}

func NewRedirectResponse(location string) Response {
	return NewResponse(nil, nil, fasthttp.StatusMovedPermanently, Header{
		Name:  "Location",
		Value: location,
	})
}

func NewValidationErrJsonResponse(error error) Response {
	errs, ok := error.(validation.Errors)
	if !ok {
		return NewErrorJSONResponse(errors.New("error must be of type validation.Errors"))
	}
	return NewJsonResponse(errs, fasthttp.StatusUnprocessableEntity, NewUnprocessableEntityErr())
}

func NewOKJsonResponse() Response {
	return NewJsonResponse("OK", fasthttp.StatusOK, nil)
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

func (r response) GetHeaders() Headers {
	return r.headers
}

func (r *response) SetHeaders(headers Headers) {
	r.headers = headers
}

type jsonResponse struct {
	data    interface{}
	error   error
	code    int
	headers Headers
}

type JsonResponseFormat struct {
	Code    int         `json:"code"`
	Payload interface{} `json:"payload"`
}

func NewJsonResponse(data interface{}, code int, error error, headers ...Header) Response {
	headers = append(headers, Header{
		Name:  ContentTypeHeaderName,
		Value: ApplicationJsonHeaderVal,
	})
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

func (r jsonResponse) GetHeaders() Headers {
	return append(r.headers, Header{ContentTypeHeaderName, ApplicationJsonHeaderVal})
}

func (r *jsonResponse) SetHeaders(headers Headers) {
	r.headers = headers
}

func NewErrorJSONResponse(e error, headers ...Header) Response {
	if e == nil {
		return NewJsonResponse(nil, fasthttp.StatusOK, e, headers...)
	}
	var errs validation.Errors
	if ok := errors.As(e, &errs); ok {
		return NewJsonResponse(errs, fasthttp.StatusUnprocessableEntity, NewUnprocessableEntityErr())
	}
	if errors.Is(e, sql.ErrNoRows) {
		return NewJsonResponse(e, fasthttp.StatusNotFound, e)
	}
	var driverErr *pq.Error
	if ok := errors.Is(e, driverErr); ok {
		if driverErr.Code == ErrLockNotAvailable {
			return NewJsonResponse("Object is being used by another transaction", fasthttp.StatusNotAcceptable, e)
		}
		if driverErr.Code == ErrRowCheckConstraint {
			return NewJsonResponse("Failed row constraint check", fasthttp.StatusNotAcceptable, e)
		}
		if driverErr.Code == ErrUniqueConstraint {
			return NewJsonResponse("Conflict", fasthttp.StatusConflict, e)
		}
	}

	nextCode := fasthttp.StatusInternalServerError
	var er erro
	if errors.As(e, &er) {
		nextCode = er.GetCode()
	}
	return NewJsonResponse(e.Error(), nextCode, e, headers...)
}
