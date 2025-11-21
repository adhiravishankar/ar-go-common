package common

import (
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// getPathParam extracts a path parameter from the URL
// For Go 1.22+ http.ServeMux with pattern matching
func GetPathParam(r *http.Request, param string) string {
	return r.PathValue(param)
}

// OptionsHandler middleware handles OPTIONS requests for CORS preflight
func OptionsHandler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodOptions {
			// CORS headers are already set by CorsMiddleware, just return 200
			w.WriteHeader(http.StatusOK)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// recoveryMiddleware recovers from panics
func RecoveryMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				w.Write([]byte("Internal Server Error"))
			}
		}()
		next.ServeHTTP(w, r)
	})
}

// Security headers middleware
func SecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("X-XSS-Protection", "1; mode=block")
		w.Header().Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
		w.Header().Set("Content-Security-Policy", "default-src 'self'")
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
		next.ServeHTTP(w, r)
	})
}

// Logging middleware for security events
func SecurityLogging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		path := r.URL.Path
		method := r.Method

		// Wrap response writer to capture status code
		lrw := &loggingResponseWriter{ResponseWriter: w, statusCode: http.StatusOK}

		next.ServeHTTP(lrw, r)

		// Log security-relevant events
		status := lrw.statusCode
		latency := time.Since(start)

		if status >= 400 {
			log.Printf("SECURITY: %s %s - Status: %d, Latency: %v, IP: %s, User-Agent: %s",
				method, path, status, latency, GetClientIP(r), r.UserAgent())
		}
	})
}

// CorsMiddleware returns a middleware that applies CORS headers for native net/http handlers.
// - `allowedOrigins`: list of origins to allow; if empty allows `*`.
// - `allowedMethods`: list of allowed methods; if empty defaults to GET,POST,PUT,DELETE,OPTIONS
// - `allowedHeaders`: list of allowed request headers; if empty will echo requested headers
// - `allowCredentials`: whether to expose Access-Control-Allow-Credentials
// - `maxAge`: seconds for Access-Control-Max-Age (0 to omit)
func CorsMiddleware(allowedOrigins []string, allowedMethods []string, allowedHeaders []string, allowCredentials bool, maxAge int) func(http.Handler) http.Handler {
	// prepare joined header values
	if len(allowedMethods) == 0 {
		allowedMethods = []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"}
	}

	if len(allowedHeaders) == 0 {
		allowedHeaders = []string{"Origin", "Content-Type", "Accept", "Authorization"}
	}

	methods := strings.Join(allowedMethods, ",")
	headers := strings.Join(allowedHeaders, ",")

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")

			// If no origin provided, just proceed
			if origin == "" {
				next.ServeHTTP(w, r)
				return
			}

			// Determine if origin is allowed
			allowOrigin := ""
			if len(allowedOrigins) == 0 {
				allowOrigin = "*"
			} else {
				for _, o := range allowedOrigins {
					if o == "*" || o == origin {
						allowOrigin = origin
						break
					}
				}
			}

			if allowOrigin == "" {
				// Not allowed origin; proceed without CORS headers (request will likely fail in browser)
				next.ServeHTTP(w, r)
				return
			}

			// Set common CORS headers
			w.Header().Set("Access-Control-Allow-Origin", allowOrigin)
			w.Header().Set("Vary", "Origin")
			w.Header().Set("Access-Control-Allow-Methods", methods)

			// If allowedHeaders not provided, echo back requested headers for preflight
			w.Header().Set("Access-Control-Allow-Headers", headers)

			if allowCredentials {
				w.Header().Set("Access-Control-Allow-Credentials", "true")
			}

			if maxAge > 0 {
				w.Header().Set("Access-Control-Max-Age", strconv.Itoa(maxAge))
			}

			// Handle preflight
			if r.Method == http.MethodOptions {
				// For preflight respond with 204 No Content
				w.WriteHeader(http.StatusNoContent)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// loggingResponseWriter wraps http.ResponseWriter to capture status code
type loggingResponseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (lrw *loggingResponseWriter) WriteHeader(code int) {
	lrw.statusCode = code
	lrw.ResponseWriter.WriteHeader(code)
}

// GetClientIP extracts the client IP from the request
func GetClientIP(r *http.Request) string {
	// Check X-Forwarded-For header first (for proxied requests)
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		return xff
	}
	// Check X-Real-IP header
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return xri
	}
	// Fall back to RemoteAddr
	return r.RemoteAddr
}
