package errors

import (
	"fmt"
	"net/http"
)

// ErrorType represents different types of errors in the system
type ErrorType string

const (
	ErrorTypeValidation   ErrorType = "validation"
	ErrorTypeNetwork      ErrorType = "network"
	ErrorTypeCacheFailure ErrorType = "cache_failure"
	ErrorTypeInternal     ErrorType = "internal"
	ErrorTypeNotFound     ErrorType = "not_found"
	ErrorTypeTimeout      ErrorType = "timeout"
)

// AppError represents a structured application error
type AppError struct {
	Type       ErrorType `json:"type"`
	Message    string    `json:"message"`
	Code       string    `json:"code"`
	HTTPStatus int       `json:"-"`
	Cause      error     `json:"-"`
}

// Error implements the error interface
func (e *AppError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("%s: %s (caused by: %v)", e.Code, e.Message, e.Cause)
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

// Unwrap returns the underlying error
func (e *AppError) Unwrap() error {
	return e.Cause
}

// New creates a new AppError
func New(errorType ErrorType, code, message string, httpStatus int) *AppError {
	return &AppError{
		Type:       errorType,
		Code:       code,
		Message:    message,
		HTTPStatus: httpStatus,
	}
}

// Wrap wraps an existing error with additional context
func Wrap(err error, errorType ErrorType, code, message string, httpStatus int) *AppError {
	return &AppError{
		Type:       errorType,
		Code:       code,
		Message:    message,
		HTTPStatus: httpStatus,
		Cause:      err,
	}
}

// Predefined errors
var (
	ErrInvalidOriginURL = New(
		ErrorTypeValidation,
		"INVALID_ORIGIN_URL",
		"The provided origin URL is invalid",
		http.StatusBadRequest,
	)

	ErrCacheKeyGeneration = New(
		ErrorTypeInternal,
		"CACHE_KEY_GENERATION_FAILED",
		"Failed to generate cache key",
		http.StatusInternalServerError,
	)

	ErrOriginRequestFailed = New(
		ErrorTypeNetwork,
		"ORIGIN_REQUEST_FAILED",
		"Failed to reach origin server",
		http.StatusBadGateway,
	)

	ErrOriginResponseRead = New(
		ErrorTypeNetwork,
		"ORIGIN_RESPONSE_READ_FAILED",
		"Failed to read response from origin server",
		http.StatusInternalServerError,
	)

	ErrRequestCreation = New(
		ErrorTypeInternal,
		"REQUEST_CREATION_FAILED",
		"Failed to create request to origin server",
		http.StatusInternalServerError,
	)
)
