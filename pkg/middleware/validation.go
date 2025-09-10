package middleware

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"reflect"
	"strconv"
	"strings"

	"go.uber.org/zap"
)

// ValidationConfig holds validation configuration
type ValidationConfig struct {
	MaxBodySize int64 // Maximum request body size in bytes
	Logger      *zap.Logger
}

// Validator interface for custom validation logic
type Validator interface {
	Validate() error
}

// ValidationError represents a validation error
type ValidationError struct {
	Field   string      `json:"field"`
	Message string      `json:"message"`
	Value   interface{} `json:"value,omitempty"`
}

// ValidationErrors represents multiple validation errors
type ValidationErrors struct {
	Errors []ValidationError `json:"errors"`
}

func (ve ValidationErrors) Error() string {
	var messages []string
	for _, err := range ve.Errors {
		messages = append(messages, fmt.Sprintf("%s: %s", err.Field, err.Message))
	}
	return strings.Join(messages, "; ")
}

// RequestValidation middleware validates request body size and JSON format
func RequestValidation(config ValidationConfig) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Skip validation for GET requests and health checks
			if r.Method == "GET" || strings.HasPrefix(r.URL.Path, "/health") {
				next.ServeHTTP(w, r)
				return
			}

			// Check content length
			if r.ContentLength > config.MaxBodySize {
				config.Logger.Warn("Request body too large",
					zap.Int64("content_length", r.ContentLength),
					zap.Int64("max_size", config.MaxBodySize),
					zap.String("path", r.URL.Path))

				http.Error(w, fmt.Sprintf("Request body too large. Maximum size: %d bytes", config.MaxBodySize),
					http.StatusRequestEntityTooLarge)
				return
			}

			// Read and validate JSON body for POST/PUT requests
			if r.Method == "POST" || r.Method == "PUT" {
				body, err := io.ReadAll(io.LimitReader(r.Body, config.MaxBodySize))
				if err != nil {
					config.Logger.Error("Failed to read request body", zap.Error(err))
					http.Error(w, "Failed to read request body", http.StatusBadRequest)
					return
				}
				r.Body.Close()

				// Validate JSON format
				if len(body) > 0 {
					var jsonData interface{}
					if err := json.Unmarshal(body, &jsonData); err != nil {
						config.Logger.Warn("Invalid JSON format",
							zap.Error(err),
							zap.String("path", r.URL.Path))

						http.Error(w, "Invalid JSON format", http.StatusBadRequest)
						return
					}
				}

				// Restore body for downstream handlers
				r.Body = io.NopCloser(bytes.NewReader(body))
			}

			next.ServeHTTP(w, r)
		})
	}
}

// ValidateStruct validates a struct using reflection and struct tags
func ValidateStruct(s interface{}) error {
	var errors []ValidationError

	v := reflect.ValueOf(s)
	t := reflect.TypeOf(s)

	// Handle pointer to struct
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
		t = t.Elem()
	}

	if v.Kind() != reflect.Struct {
		return fmt.Errorf("expected struct, got %s", v.Kind())
	}

	for i := 0; i < v.NumField(); i++ {
		field := v.Field(i)
		fieldType := t.Field(i)

		// Skip unexported fields
		if !field.CanInterface() {
			continue
		}

		// Get validation tag
		validateTag := fieldType.Tag.Get("validate")
		if validateTag == "" {
			continue
		}

		// Parse validation rules
		rules := strings.Split(validateTag, ",")
		for _, rule := range rules {
			rule = strings.TrimSpace(rule)
			if err := validateField(fieldType.Name, field, rule); err != nil {
				errors = append(errors, *err)
			}
		}
	}

	if len(errors) > 0 {
		return ValidationErrors{Errors: errors}
	}

	return nil
}

// validateField validates a single field against a rule
func validateField(fieldName string, field reflect.Value, rule string) *ValidationError {
	parts := strings.Split(rule, "=")
	ruleName := parts[0]
	var ruleValue string
	if len(parts) > 1 {
		ruleValue = parts[1]
	}

	switch ruleName {
	case "required":
		if isEmptyValue(field) {
			return &ValidationError{
				Field:   fieldName,
				Message: "field is required",
				Value:   field.Interface(),
			}
		}

	case "min":
		if ruleValue == "" {
			return &ValidationError{
				Field:   fieldName,
				Message: "min rule requires a value",
			}
		}

		minVal, err := strconv.ParseFloat(ruleValue, 64)
		if err != nil {
			return &ValidationError{
				Field:   fieldName,
				Message: "invalid min value",
			}
		}

		if field.Kind() == reflect.String {
			if float64(len(field.String())) < minVal {
				return &ValidationError{
					Field:   fieldName,
					Message: fmt.Sprintf("minimum length is %s", ruleValue),
					Value:   field.Interface(),
				}
			}
		} else if field.CanFloat() {
			if field.Float() < minVal {
				return &ValidationError{
					Field:   fieldName,
					Message: fmt.Sprintf("minimum value is %s", ruleValue),
					Value:   field.Interface(),
				}
			}
		}

	case "max":
		if ruleValue == "" {
			return &ValidationError{
				Field:   fieldName,
				Message: "max rule requires a value",
			}
		}

		maxVal, err := strconv.ParseFloat(ruleValue, 64)
		if err != nil {
			return &ValidationError{
				Field:   fieldName,
				Message: "invalid max value",
			}
		}

		if field.Kind() == reflect.String {
			if float64(len(field.String())) > maxVal {
				return &ValidationError{
					Field:   fieldName,
					Message: fmt.Sprintf("maximum length is %s", ruleValue),
					Value:   field.Interface(),
				}
			}
		} else if field.CanFloat() {
			if field.Float() > maxVal {
				return &ValidationError{
					Field:   fieldName,
					Message: fmt.Sprintf("maximum value is %s", ruleValue),
					Value:   field.Interface(),
				}
			}
		}

	case "gt":
		if ruleValue == "" {
			return &ValidationError{
				Field:   fieldName,
				Message: "gt rule requires a value",
			}
		}

		gtVal, err := strconv.ParseFloat(ruleValue, 64)
		if err != nil {
			return &ValidationError{
				Field:   fieldName,
				Message: "invalid gt value",
			}
		}

		if field.CanFloat() {
			if field.Float() <= gtVal {
				return &ValidationError{
					Field:   fieldName,
					Message: fmt.Sprintf("value must be greater than %s", ruleValue),
					Value:   field.Interface(),
				}
			}
		}

	case "oneof":
		if ruleValue == "" {
			return &ValidationError{
				Field:   fieldName,
				Message: "oneof rule requires values",
			}
		}

		allowedValues := strings.Split(ruleValue, " ")
		fieldStr := fmt.Sprintf("%v", field.Interface())

		found := false
		for _, allowed := range allowedValues {
			if fieldStr == allowed {
				found = true
				break
			}
		}

		if !found {
			return &ValidationError{
				Field:   fieldName,
				Message: fmt.Sprintf("value must be one of: %s", ruleValue),
				Value:   field.Interface(),
			}
		}
	}

	return nil
}

// isEmptyValue checks if a value is considered empty
func isEmptyValue(v reflect.Value) bool {
	switch v.Kind() {
	case reflect.String, reflect.Array:
		return v.Len() == 0
	case reflect.Map, reflect.Slice:
		return v.Len() == 0 || v.IsNil()
	case reflect.Bool:
		return !v.Bool()
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return v.Int() == 0
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		return v.Uint() == 0
	case reflect.Float32, reflect.Float64:
		return v.Float() == 0
	case reflect.Interface, reflect.Ptr:
		return v.IsNil()
	}
	return false
}

// ErrorResponse creates a standardized error response
func ErrorResponse(w http.ResponseWriter, statusCode int, message string, details interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)

	response := map[string]interface{}{
		"error":  message,
		"status": statusCode,
	}

	if details != nil {
		response["details"] = details
	}

	json.NewEncoder(w).Encode(response)
}
