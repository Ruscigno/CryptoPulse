package errors

import (
	"fmt"
	"net/http"
	"time"
)

// ErrorCode represents different types of errors
type ErrorCode string

const (
	// Client errors (4xx)
	ErrCodeBadRequest          ErrorCode = "BAD_REQUEST"
	ErrCodeUnauthorized        ErrorCode = "UNAUTHORIZED"
	ErrCodeForbidden           ErrorCode = "FORBIDDEN"
	ErrCodeNotFound            ErrorCode = "NOT_FOUND"
	ErrCodeConflict            ErrorCode = "CONFLICT"
	ErrCodeValidation          ErrorCode = "VALIDATION_ERROR"
	ErrCodeRateLimit           ErrorCode = "RATE_LIMIT_EXCEEDED"
	ErrCodeRequestTooLarge     ErrorCode = "REQUEST_TOO_LARGE"

	// Server errors (5xx)
	ErrCodeInternal            ErrorCode = "INTERNAL_ERROR"
	ErrCodeServiceUnavailable  ErrorCode = "SERVICE_UNAVAILABLE"
	ErrCodeTimeout             ErrorCode = "TIMEOUT"
	ErrCodeCircuitBreakerOpen  ErrorCode = "CIRCUIT_BREAKER_OPEN"

	// Business logic errors
	ErrCodeInsufficientFunds   ErrorCode = "INSUFFICIENT_FUNDS"
	ErrCodeInvalidMarket       ErrorCode = "INVALID_MARKET"
	ErrCodeOrderNotFound       ErrorCode = "ORDER_NOT_FOUND"
	ErrCodePositionNotFound    ErrorCode = "POSITION_NOT_FOUND"
	ErrCodeInvalidOrderState   ErrorCode = "INVALID_ORDER_STATE"

	// External service errors
	ErrCodeDydxAPIError        ErrorCode = "DYDX_API_ERROR"
	ErrCodeDatabaseError       ErrorCode = "DATABASE_ERROR"
	ErrCodeWalletError         ErrorCode = "WALLET_ERROR"
	ErrCodeTransactionFailed   ErrorCode = "TRANSACTION_FAILED"
)

// AppError represents an application error with additional context
type AppError struct {
	Code       ErrorCode              `json:"code"`
	Message    string                 `json:"message"`
	Details    string                 `json:"details,omitempty"`
	Cause      error                  `json:"-"`
	Timestamp  time.Time              `json:"timestamp"`
	RequestID  string                 `json:"request_id,omitempty"`
	UserID     string                 `json:"user_id,omitempty"`
	Metadata   map[string]interface{} `json:"metadata,omitempty"`
	Retryable  bool                   `json:"retryable"`
	HTTPStatus int                    `json:"-"`
}

// Error implements the error interface
func (e *AppError) Error() string {
	if e.Details != "" {
		return fmt.Sprintf("%s: %s - %s", e.Code, e.Message, e.Details)
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

// Unwrap returns the underlying cause error
func (e *AppError) Unwrap() error {
	return e.Cause
}

// IsRetryable returns whether the error is retryable
func (e *AppError) IsRetryable() bool {
	return e.Retryable
}

// WithCause adds a cause error
func (e *AppError) WithCause(cause error) *AppError {
	e.Cause = cause
	return e
}

// WithDetails adds additional details
func (e *AppError) WithDetails(details string) *AppError {
	e.Details = details
	return e
}

// WithRequestID adds a request ID
func (e *AppError) WithRequestID(requestID string) *AppError {
	e.RequestID = requestID
	return e
}

// WithUserID adds a user ID
func (e *AppError) WithUserID(userID string) *AppError {
	e.UserID = userID
	return e
}

// WithMetadata adds metadata
func (e *AppError) WithMetadata(key string, value interface{}) *AppError {
	if e.Metadata == nil {
		e.Metadata = make(map[string]interface{})
	}
	e.Metadata[key] = value
	return e
}

// NewAppError creates a new application error
func NewAppError(code ErrorCode, message string) *AppError {
	return &AppError{
		Code:       code,
		Message:    message,
		Timestamp:  time.Now(),
		HTTPStatus: getHTTPStatusForCode(code),
		Retryable:  isRetryableCode(code),
	}
}

// getHTTPStatusForCode returns the appropriate HTTP status code for an error code
func getHTTPStatusForCode(code ErrorCode) int {
	switch code {
	case ErrCodeBadRequest, ErrCodeValidation:
		return http.StatusBadRequest
	case ErrCodeUnauthorized:
		return http.StatusUnauthorized
	case ErrCodeForbidden:
		return http.StatusForbidden
	case ErrCodeNotFound, ErrCodeOrderNotFound, ErrCodePositionNotFound:
		return http.StatusNotFound
	case ErrCodeConflict, ErrCodeInvalidOrderState:
		return http.StatusConflict
	case ErrCodeRateLimit:
		return http.StatusTooManyRequests
	case ErrCodeRequestTooLarge:
		return http.StatusRequestEntityTooLarge
	case ErrCodeServiceUnavailable, ErrCodeCircuitBreakerOpen:
		return http.StatusServiceUnavailable
	case ErrCodeTimeout:
		return http.StatusGatewayTimeout
	default:
		return http.StatusInternalServerError
	}
}

// isRetryableCode returns whether an error code represents a retryable error
func isRetryableCode(code ErrorCode) bool {
	switch code {
	case ErrCodeTimeout, ErrCodeServiceUnavailable, ErrCodeInternal, 
		 ErrCodeDydxAPIError, ErrCodeDatabaseError, ErrCodeTransactionFailed:
		return true
	default:
		return false
	}
}

// Predefined errors for common scenarios
var (
	ErrInvalidRequest = NewAppError(ErrCodeBadRequest, "Invalid request")
	ErrUnauthorized   = NewAppError(ErrCodeUnauthorized, "Unauthorized")
	ErrForbidden      = NewAppError(ErrCodeForbidden, "Forbidden")
	ErrNotFound       = NewAppError(ErrCodeNotFound, "Resource not found")
	ErrInternal       = NewAppError(ErrCodeInternal, "Internal server error")
	ErrTimeout        = NewAppError(ErrCodeTimeout, "Request timeout")
	ErrRateLimit      = NewAppError(ErrCodeRateLimit, "Rate limit exceeded")
	
	// Business logic errors
	ErrInsufficientFunds = NewAppError(ErrCodeInsufficientFunds, "Insufficient funds")
	ErrInvalidMarket     = NewAppError(ErrCodeInvalidMarket, "Invalid market")
	ErrOrderNotFound     = NewAppError(ErrCodeOrderNotFound, "Order not found")
	ErrPositionNotFound  = NewAppError(ErrCodePositionNotFound, "Position not found")
	ErrInvalidOrderState = NewAppError(ErrCodeInvalidOrderState, "Invalid order state")
)

// WrapError wraps an existing error with additional context
func WrapError(err error, code ErrorCode, message string) *AppError {
	if err == nil {
		return nil
	}
	
	// If it's already an AppError, preserve the original code if not specified
	if appErr, ok := err.(*AppError); ok {
		if code == "" {
			code = appErr.Code
		}
		return &AppError{
			Code:       code,
			Message:    message,
			Cause:      appErr,
			Timestamp:  time.Now(),
			HTTPStatus: getHTTPStatusForCode(code),
			Retryable:  isRetryableCode(code),
		}
	}
	
	return &AppError{
		Code:       code,
		Message:    message,
		Cause:      err,
		Timestamp:  time.Now(),
		HTTPStatus: getHTTPStatusForCode(code),
		Retryable:  isRetryableCode(code),
	}
}

// IsAppError checks if an error is an AppError
func IsAppError(err error) bool {
	_, ok := err.(*AppError)
	return ok
}

// GetAppError extracts an AppError from an error chain
func GetAppError(err error) *AppError {
	if err == nil {
		return nil
	}
	
	if appErr, ok := err.(*AppError); ok {
		return appErr
	}
	
	// Check if it's wrapped
	if unwrapped := unwrapError(err); unwrapped != nil {
		return GetAppError(unwrapped)
	}
	
	return nil
}

// unwrapError unwraps an error if it implements the Unwrap method
func unwrapError(err error) error {
	type unwrapper interface {
		Unwrap() error
	}
	
	if u, ok := err.(unwrapper); ok {
		return u.Unwrap()
	}
	
	return nil
}

// ErrorResponse represents an HTTP error response
type ErrorResponse struct {
	Error     string                 `json:"error"`
	Code      ErrorCode              `json:"code"`
	Message   string                 `json:"message"`
	Details   string                 `json:"details,omitempty"`
	Timestamp time.Time              `json:"timestamp"`
	RequestID string                 `json:"request_id,omitempty"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
}

// ToErrorResponse converts an AppError to an ErrorResponse
func (e *AppError) ToErrorResponse() ErrorResponse {
	return ErrorResponse{
		Error:     "error",
		Code:      e.Code,
		Message:   e.Message,
		Details:   e.Details,
		Timestamp: e.Timestamp,
		RequestID: e.RequestID,
		Metadata:  e.Metadata,
	}
}

// ValidationError represents a validation error with field-specific details
type ValidationError struct {
	*AppError
	Fields []FieldError `json:"fields"`
}

// FieldError represents an error for a specific field
type FieldError struct {
	Field   string      `json:"field"`
	Message string      `json:"message"`
	Value   interface{} `json:"value,omitempty"`
}

// NewValidationError creates a new validation error
func NewValidationError(message string, fields []FieldError) *ValidationError {
	return &ValidationError{
		AppError: NewAppError(ErrCodeValidation, message),
		Fields:   fields,
	}
}

// AddField adds a field error to the validation error
func (ve *ValidationError) AddField(field, message string, value interface{}) *ValidationError {
	ve.Fields = append(ve.Fields, FieldError{
		Field:   field,
		Message: message,
		Value:   value,
	})
	return ve
}

// HasFields returns true if the validation error has field errors
func (ve *ValidationError) HasFields() bool {
	return len(ve.Fields) > 0
}
