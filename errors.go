package common

import (
	"encoding/json"
	"fmt"
	"net/http"
)

// ErrorResponse represents a standard error response
type ErrorResponse struct {
	Error   string `json:"error"`
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// RespondWithError provides standardized error handling with proper HTTP codes
func RespondWithError(w http.ResponseWriter, code int, err error) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(ErrorResponse{
		Error:   err.Error(),
		Code:    code,
		Message: getErrorMessage(code),
	})
}

// RespondWithValidationError provides specific validation error handling
func RespondWithValidationError(w http.ResponseWriter, field string, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(400)
	json.NewEncoder(w).Encode(ErrorResponse{
		Error:   fmt.Sprintf("validation failed for field '%s': %s", field, message),
		Code:    400,
		Message: "Validation Error",
	})
}

// RespondWithJSON sends a JSON response
func RespondWithJSON(w http.ResponseWriter, code int, payload interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(payload)
}

func getErrorMessage(code int) string {
	switch code {
	case 400:
		return "Bad Request"
	case 404:
		return "Not Found"
	case 500:
		return "Internal Server Error"
	default:
		return "Error"
	}
}
