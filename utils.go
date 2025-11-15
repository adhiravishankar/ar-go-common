package common

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"
)

// ValidateRequiredFields checks if required fields are not empty
func ValidateRequiredFields(w http.ResponseWriter, fields map[string]string) bool {
	for field, value := range fields {
		if strings.TrimSpace(value) == "" {
			RespondWithValidationError(w, field, "is required")
			return false
		}
	}
	return true
}

// ValidateAndBindJSON validates and binds JSON input with proper error handling
func ValidateAndBindJSON(w http.ResponseWriter, r *http.Request, target interface{}) bool {
	if err := json.NewDecoder(r.Body).Decode(target); err != nil {
		RespondWithError(w, 400, err)
		return false
	}
	return true
}

// HealthCheckResponse represents a health check response
type HealthCheckResponse struct {
	Status    string `json:"status"`
	Timestamp string `json:"timestamp"`
}

// HealthCheck provides a standard health check endpoint
func HealthCheck(w http.ResponseWriter, r *http.Request) {
	RespondWithJSON(w, 200, HealthCheckResponse{
		Status:    "healthy",
		Timestamp: time.Now().Format(time.RFC3339),
	})
}
