package common

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"golang.org/x/crypto/argon2"
)

var (
	ErrInvalidHash         = errors.New("the encoded hash is not in the correct format")
	ErrIncompatibleVersion = errors.New("incompatible version of argon2")
	ErrInvalidInput        = errors.New("invalid input provided")
)

// Improved Argon2 parameters for better security
var defaultPasswordParams = &PasswordParams{
	memory:      64 * 1024,
	iterations:  10,
	parallelism: 1,
	saltLength:  16,
	keyLength:   32,
}

type PasswordParams struct {
	memory      uint32
	iterations  uint32
	parallelism uint8
	saltLength  uint32
	keyLength   uint32
}

// SanitizeInput removes potentially dangerous characters
func SanitizeInput(input string) string {
	// Remove null bytes and control characters
	input = strings.Map(func(r rune) rune {
		if r < 32 && r != 9 && r != 10 && r != 13 { // Allow tab, newline, carriage return
			return -1
		}
		return r
	}, input)

	return strings.TrimSpace(input)
}

// validateJWTSecret ensures the JWT secret meets security requirements
func ValidateJWTSecret() error {
	secret := os.Getenv("JWT_SECRET")
	if secret == "" {
		return fmt.Errorf("JWT_SECRET environment variable is required")
	}

	if len(secret) < 32 {
		return fmt.Errorf("JWT_SECRET must be at least 32 characters long")
	}

	return nil
}

func Authenticate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Validate JWT secret first
		if err := ValidateJWTSecret(); err != nil {
			log.Printf("JWT secret validation failed: %v", err)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(500)
			json.NewEncoder(w).Encode(map[string]string{"error": "Server configuration error"})
			return
		}

		// Get Authorization header
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(401)
			json.NewEncoder(w).Encode(map[string]string{"error": "Authorization required"})
			return
		}

		// Check if it starts with "Bearer "
		const bearerPrefix = "Bearer "
		if !strings.HasPrefix(authHeader, bearerPrefix) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(401)
			json.NewEncoder(w).Encode(map[string]string{"error": "Invalid authorization format"})
			return
		}

		// Extract the token
		tokenString := strings.TrimPrefix(authHeader, bearerPrefix)

		// Parse and validate the token with improved error handling
		token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
			// Validate the signing method
			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
			}
			return []byte(os.Getenv("JWT_SECRET")), nil
		})

		if err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(401)
			switch {
			case errors.Is(err, jwt.ErrTokenMalformed):
				json.NewEncoder(w).Encode(map[string]string{"error": "Invalid token"})
			case errors.Is(err, jwt.ErrTokenSignatureInvalid):
				json.NewEncoder(w).Encode(map[string]string{"error": "Invalid token"})
			case errors.Is(err, jwt.ErrTokenExpired):
				json.NewEncoder(w).Encode(map[string]string{"error": "Token expired"})
			case errors.Is(err, jwt.ErrTokenNotValidYet):
				json.NewEncoder(w).Encode(map[string]string{"error": "Token not valid yet"})
			default:
				json.NewEncoder(w).Encode(map[string]string{"error": "Invalid token"})
			}
			return
		}

		if !token.Valid {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(401)
			json.NewEncoder(w).Encode(map[string]string{"error": "Invalid token"})
			return
		}

		// Extract and validate claims
		if claims, ok := token.Claims.(jwt.MapClaims); ok {
			expiresAt, err := claims.GetExpirationTime()
			if err != nil {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(401)
				json.NewEncoder(w).Encode(map[string]string{"error": "Invalid token claims"})
				return
			}

			if expiresAt.Before(time.Now()) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(401)
				json.NewEncoder(w).Encode(map[string]string{"error": "Token expired"})
				return
			}

			issuedAt, err := claims.GetIssuedAt()
			if err != nil {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(401)
				json.NewEncoder(w).Encode(map[string]string{"error": "Invalid token claims"})
				return
			}

			if issuedAt.After(time.Now()) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(401)
				json.NewEncoder(w).Encode(map[string]string{"error": "Token not valid yet"})
				return
			}

			userID, err := claims.GetSubject()
			if err != nil {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(401)
				json.NewEncoder(w).Encode(map[string]string{"error": "Invalid token claims"})
				return
			}

			// Validate user ID format
			if _, err := uuid.Parse(userID); err != nil {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(401)
				json.NewEncoder(w).Encode(map[string]string{"error": "Invalid token claims"})
				return
			}

			// Set the user ID in the context for later use
			r = SetUserID(r, userID)
			next.ServeHTTP(w, r)
		} else {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(401)
			json.NewEncoder(w).Encode(map[string]string{"error": "Invalid token claims"})
			return
		}
	})
}

func GenerateFromPassword(password string, p *PasswordParams) (encodedHash string, err error) {
	// Generate a cryptographically secure random salt.
	salt, err := GenerateRandomBytes(p.saltLength)
	if err != nil {
		return "", err
	}

	// Pass the plaintext password, salt and parameters to the argon2.IDKey
	// function. This will generate a hash of the password using the Argon2id
	// variant.
	hash := argon2.IDKey([]byte(password), salt, p.iterations, p.memory, p.parallelism, p.keyLength)

	// Base64 encode the salt and hashed password.
	b64Salt := base64.RawStdEncoding.EncodeToString(salt)
	b64Hash := base64.RawStdEncoding.EncodeToString(hash)

	// Return a string using the standard encoded hash representation.
	encodedHash = fmt.Sprintf("$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s", argon2.Version, p.memory, p.iterations, p.parallelism, b64Salt, b64Hash)

	return encodedHash, nil
}

func GenerateRandomBytes(n uint32) ([]byte, error) {
	b := make([]byte, n)
	_, err := rand.Read(b)
	if err != nil {
		return nil, err
	}

	return b, nil
}
