package core

import (
	"github.com/pkg/errors"
	"net/http"
	"strings"
)

type Error interface {
	GetCode() int
	Error() string
}

type erro struct {
	Code    int
	Message string
	Err     error
}

type StackTracer interface {
	StackTrace() errors.StackTrace
}

func JoinStrings(def string, strs ...string) string {
	if len(strs) < 1 {
		return def
	}
	return strings.Join(strs, " ")
}

func NewLucierror(code int, message string) error {
	return errors.Wrap(erro{Code: code, Message: message}, "Error")
}

func Wrap(err error) error {
	if err == nil {
		return nil
	}
	return errors.Wrap(erro{Code: http.StatusInternalServerError, Message: err.Error(), Err: err}, "Error")
}

func wrapErr(err Error) error {
	return errors.Wrap(erro{Code: err.GetCode(), Message: err.Error(), Err: err}, "Error")
}

func (e erro) Unwrap() error {
	return e.Err
}

func (e erro) GetCode() int {
	return e.Code
}

func (e erro) Error() string {
	return e.Message
}

//======================================================================================================================

type BadRequest struct {
	message string
}

func (e BadRequest) GetCode() int {
	return http.StatusBadRequest
}

func (e BadRequest) Error() string {
	return e.message
}

func BadRequestErr(message ...string) error {
	return wrapErr(BadRequest{message: JoinStrings("Bad request", message...)})
}

//======================================================================================================================

type UnprocessableEntityErr struct {
	message string
}

func (e UnprocessableEntityErr) GetCode() int {
	return http.StatusUnprocessableEntity
}

func (e UnprocessableEntityErr) Error() string {
	return e.message
}

func NewUnprocessableEntityErr(message ...string) error {
	return wrapErr(UnprocessableEntityErr{message: JoinStrings("Unprocessable entity", message...)})
}

//======================================================================================================================

type ObjectOnLock struct {
	message string
}

func (e ObjectOnLock) GetCode() int {
	return http.StatusNotAcceptable
}

func (e ObjectOnLock) Error() string {
	return e.message
}

func ObjectOnLockErr(message ...string) error {
	return wrapErr(ObjectOnLock{message: JoinStrings("Object on lock", message...)})
}

//======================================================================================================================

type AccessDenied struct {
	message string
}

func (e AccessDenied) GetCode() int {
	return http.StatusForbidden
}

func (e AccessDenied) Error() string {
	return e.message
}

func AccessDeniedErr(message ...string) error {
	return wrapErr(AccessDenied{message: JoinStrings("Access denied", message...)})
}

func InvalidCredentialsErr(message ...string) error {
	return wrapErr(AccessDenied{message: JoinStrings("Invalid credentials", message...)})
}

//======================================================================================================================

type Unauthorized struct {
	message string
}

func (e Unauthorized) GetCode() int {
	return http.StatusUnauthorized
}

func (e Unauthorized) Error() string {
	return e.message
}

func UnauthorizedErr(message ...string) error {
	return wrapErr(Unauthorized{message: JoinStrings("Unauthorized", message...)})
}

func AuthorizationRequiredErr(message ...string) error {
	return wrapErr(Unauthorized{message: JoinStrings("Authorization required", message...)})
}

func AuthorizationExpiredErr(message ...string) error {
	return wrapErr(Unauthorized{message: JoinStrings("Authorization expired", message...)})
}

func InvalidGrantErr(message ...string) error {
	return wrapErr(Unauthorized{message: JoinStrings("Invalid grant", message...)})
}

func UnknownGrantTypeErr(message ...string) error {
	return wrapErr(Unauthorized{message: JoinStrings("Unknown grant type", message...)})
}

//======================================================================================================================

type ObjectNotFound struct {
	message string
}

func (e ObjectNotFound) GetCode() int {
	return http.StatusNotFound
}

func (e ObjectNotFound) Error() string {
	return e.message
}

func ObjectNotFoundErr(message ...string) error {
	return wrapErr(ObjectNotFound{message: JoinStrings("Not found", message...)})
}

//======================================================================================================================

type Conflict struct {
	message string
}

func (e Conflict) GetCode() int {
	return http.StatusConflict
}

func (e Conflict) Error() string {
	return e.message
}

func ConflictErr(message ...string) error {
	return wrapErr(Conflict{message: JoinStrings("Conflict", message...)})
}

//======================================================================================================================

type PreconditionFailed struct {
	message string
}

func (e PreconditionFailed) GetCode() int {
	return http.StatusPreconditionFailed
}

func (e PreconditionFailed) Error() string {
	return e.message
}

func PreconditionFailedErr(message ...string) error {
	return wrapErr(PreconditionFailed{message: JoinStrings("Precondition", message...)})
}
