package main

import (
	"crypto/rand"
	"fmt"
	"log"
	"net/http"
	"regexp"
	"strings"
	"time"
	"unicode"

	"github.com/adhiravishankar/fh-go-backends/common"
	"github.com/google/uuid"
	"go.mongodb.org/mongo-driver/bson"
)

// Email validation regex
var emailRegex = regexp.MustCompile(`^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`)

type RegisterForm struct {
	Email    string `json:"email" binding:"required"`    // The email of the user
	Password string `json:"password" binding:"required"` // The password of the user
	Name     string `json:"name" binding:"required"`     // The name of the user
}

// validateEmail checks if the email meets security requirements
func validateEmail(email string) error {
	if len(email) < 6 {
		return fmt.Errorf("email must be at least 6 characters long")
	}

	if !emailRegex.MatchString(email) {
		return fmt.Errorf("email can only contain letters, numbers, underscores, and hyphens")
	}

	return nil
}

// validatePassword checks if the password meets security requirements
func validatePassword(password string) error {
	if len(password) < 16 {
		return fmt.Errorf("password must be at least 16 characters long")
	}

	if len(password) > 128 {
		return fmt.Errorf("password must be less than 128 characters")
	}

	var (
		hasUpper   bool
		hasLower   bool
		hasNumber  bool
		hasSpecial bool
	)

	for _, char := range password {
		switch {
		case unicode.IsUpper(char):
			hasUpper = true
		case unicode.IsLower(char):
			hasLower = true
		case unicode.IsNumber(char):
			hasNumber = true
		case unicode.IsPunct(char) || unicode.IsSymbol(char):
			hasSpecial = true
		}
	}

	if !hasUpper || !hasLower || !hasNumber || !hasSpecial {
		return fmt.Errorf("password must contain at least one uppercase letter, one lowercase letter, one number, and one special character")
	}

	// Check for common weak patterns
	weakPatterns := []string{"password", "123456", "qwerty", "admin", "letmein"}
	lowerPassword := strings.ToLower(password)
	for _, pattern := range weakPatterns {
		if strings.Contains(lowerPassword, pattern) {
			return fmt.Errorf("password contains common weak patterns")
		}
	}

	return nil
}

// validateVerificationToken validates that a token is exactly 8 digits
func validateVerificationToken(token string) error {
	if len(token) != 8 {
		return fmt.Errorf("verification token must be exactly 8 digits")
	}

	for _, char := range token {
		if char < '0' || char > '9' {
			return fmt.Errorf("verification token must contain only numbers")
		}
	}

	return nil
}

// generateVerificationToken generates a cryptographically secure 8-digit verification token
func generateVerificationToken() (string, error) {
	// Generate a random 8-digit number (10000000 to 99999999)
	bytes := make([]byte, 4) // 32 bits should be enough for good randomness
	if _, err := rand.Read(bytes); err != nil {
		return "", fmt.Errorf("failed to generate random bytes: %w", err)
	}
	
	// Convert bytes to uint32 and constrain to 8-digit range
	randomNum := uint32(bytes[0])<<24 | uint32(bytes[1])<<16 | uint32(bytes[2])<<8 | uint32(bytes[3])
	
	// Ensure it's in the range 10000000-99999999 (8 digits)
	token := 10000000 + (randomNum % 90000000)
	
	return fmt.Sprintf("%08d", token), nil
}

func Register(w http.ResponseWriter, r *http.Request) {
	collection := database.Collection("users")

	// Validate JWT secret first
	if err := common.ValidateJWTSecret(); err != nil {
		log.Printf("JWT secret validation failed: %v", err)
		common.RespondWithJSON(w, 500, map[string]string{"error": "Server configuration error"})
		return
	}

	// Get the request body
	var form RegisterForm
	if !common.ValidateAndBindJSON(w, r, &form) {
		return
	}

	// Sanitize inputs
	form.Email = sanitizeInput(form.Email)
	form.Name = sanitizeInput(form.Name)

	// Validate username
	if err := validateEmail(form.Email); err != nil {
		w.WriteHeader(400)
		w.Write([]byte(err.Error()))
		return
	}

	// Validate password complexity
	if err := validatePassword(form.Password); err != nil {
		w.WriteHeader(400)
		w.Write([]byte(err.Error()))
		return
	}

	id, err := uuid.NewV7()
	if err != nil {
		log.Printf("Failed to generate UUID: %v", err)
		w.WriteHeader(500)
		w.Write([]byte("Server error"))
		return
	}

	// Generate verification token
	verificationToken, err := generateVerificationToken()
	if err != nil {
		log.Printf("Failed to generate verification token: %v", err)
		w.WriteHeader(500)
		w.Write([]byte("Server error"))
		return
	}

	// Hash the password before storing
	hashedPassword, err := GenerateFromPassword(form.Password, defaultPasswordParams)
	if err != nil {
		log.Printf("Failed to hash password: %v", err)
		w.WriteHeader(500)
		w.Write([]byte("Server error"))
		return
	}

	user := User{
		ID:            id.String(),
		Email:         form.Email,
		Password:      hashedPassword,
		Name:          form.Name,
		CreatedAt:     time.Now(),
		LoginAttempts: 0,
		IsVerified:    false,
		VerifiedAt:    nil,
	}

	// Check if username already exists (use generic error message)
	var existingUser User
	err = collection.FindOne(r.Context(), bson.M{"email": form.Email}).Decode(&existingUser)
	if err == nil {
		common.RespondWithJSON(w, 400, map[string]string{"error": "Registration failed"})
		return
	}

	_, err = collection.InsertOne(r.Context(), user)
	if err != nil {
		log.Printf("Failed to insert user: %v", err)
		common.RespondWithJSON(w, 500, map[string]string{"error": "Server error"})
		return
	}

	// Create email verification record
	if err := CreateEmailVerification(user.ID, user.Email, verificationToken); err != nil {
		log.Printf("Failed to create email verification record: %v", err)
		common.RespondWithJSON(w, 500, map[string]string{"error": "Server error"})
		return
	}

	// Send verification email
	if err := SendVerificationEmail(user.Email, user.Name, "templates/verify.html", verificationToken); err != nil {
		log.Printf("Failed to send verification email: %v", err)
		// Don't fail the registration if email sending fails
		// The user is still created and can request a new verification email
	}

	common.RespondWithJSON(w, 200, map[string]string{
		"message": "Registration successful. Please check your email to verify your account.",
		"email":   user.Email,
	})
}
