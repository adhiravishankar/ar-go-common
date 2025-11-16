package common

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/adhiravishankar/fh-go-backends/common"
	"github.com/google/uuid"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

type ForgotPasswordForm struct {
	Email string `json:"email" binding:"required"` // The email of the user
}

type ResetPasswordForm struct {
	Token       string `json:"token" binding:"required"`        // The password reset token
	NewPassword string `json:"new_password" binding:"required"` // The new password
}

// PasswordReset represents a password reset request in the database
type PasswordReset struct {
	ID        string     `json:"id" bson:"_id"`                // Unique ID for the reset request
	UserID    string     `json:"user_id" bson:"user_id"`       // ID of the user requesting reset
	Email     string     `json:"email" bson:"email"`           // Email of the user (for easier queries)
	Token     string     `json:"token" bson:"token"`           // The reset token
	ExpiresAt time.Time  `json:"expires_at" bson:"expires_at"` // When the token expires
	CreatedAt time.Time  `json:"created_at" bson:"created_at"` // When the reset was requested
	Used      bool       `json:"used" bson:"used"`             // Whether the token has been used
	UsedAt    *time.Time `json:"used_at" bson:"used_at"`       // When the token was used
}

// generatePasswordResetToken generates a cryptographically secure password reset token
func generatePasswordResetToken() (string, error) {
	bytes := make([]byte, 32) // 256 bits
	if _, err := rand.Read(bytes); err != nil {
		return "", fmt.Errorf("failed to generate random bytes: %w", err)
	}
	return hex.EncodeToString(bytes), nil
}

// ForgotPassword handles forgot password requests
func ForgotPassword(w http.ResponseWriter, r *http.Request) {
	usersCollection := database.Collection("users")
	resetsCollection := database.Collection("password_resets")

	var form ForgotPasswordForm
	if !common.ValidateAndBindJSON(w, r, &form) {
		return
	}

	// Sanitize email input
	form.Email = sanitizeInput(form.Email)

	if form.Email == "" {
		common.RespondWithJSON(w, 400, map[string]string{"error": "Email is required"})
		return
	}

	// Validate email format
	if err := validateEmail(form.Email); err != nil {
		common.RespondWithJSON(w, 400, map[string]string{"error": "Invalid email format"})
		return
	}

	// Find user by email
	var user User
	err := usersCollection.FindOne(r.Context(), bson.M{"email": form.Email}).Decode(&user)

	// Always return success to prevent email enumeration
	// Don't reveal whether the email exists or not
	successResponse := map[string]string{
		"message": "If an account with that email exists, we've sent a password reset link to it.",
	}

	if err != nil {
		if err == mongo.ErrNoDocuments {
			// Email doesn't exist, but return success to prevent enumeration
			common.RespondWithJSON(w, 200, successResponse)
			return
		}
		log.Printf("Failed to find user by email: %v", err)
		common.RespondWithJSON(w, 500, map[string]string{"error": "Server error"})
		return
	}

	// Check if user is verified
	if !user.IsVerified {
		// Don't send reset email to unverified accounts
		common.RespondWithJSON(w, 200, successResponse)
		return
	}

	// Generate password reset token
	resetToken, err := generatePasswordResetToken()
	if err != nil {
		log.Printf("Failed to generate password reset token: %v", err)
		common.RespondWithJSON(w, 500, map[string]string{"error": "Server error"})
		return
	}

	// Generate unique ID for the reset request
	resetID, err := uuid.NewV7()
	if err != nil {
		log.Printf("Failed to generate reset ID: %v", err)
		common.RespondWithJSON(w, 500, map[string]string{"error": "Server error"})
		return
	}

	// Create password reset record
	now := time.Now()
	passwordReset := PasswordReset{
		ID:        resetID.String(),
		UserID:    user.ID,
		Email:     user.Email,
		Token:     resetToken,
		ExpiresAt: now.Add(1 * time.Hour), // Token expires in 1 hour
		CreatedAt: now,
		Used:      false,
		UsedAt:    nil,
	}

	// Insert the reset record
	_, err = resetsCollection.InsertOne(r.Context(), passwordReset)
	if err != nil {
		log.Printf("Failed to create password reset record: %v", err)
		common.RespondWithJSON(w, 500, map[string]string{"error": "Server error"})
		return
	}

	// Send password reset email
	if err := SendPasswordResetEmail(user.Email, user.Name, resetToken); err != nil {
		log.Printf("Failed to send password reset email: %v", err)
		// Don't fail the request if email sending fails, but log it
	}

	common.RespondWithJSON(w, 200, successResponse)
}

// ResetPassword handles password reset with token
func ResetPassword(w http.ResponseWriter, r *http.Request) {
	usersCollection := database.Collection("users")
	resetsCollection := database.Collection("password_resets")

	var form ResetPasswordForm
	if !common.ValidateAndBindJSON(w, r, &form) {
		return
	}

	// Sanitize inputs
	form.Token = sanitizeInput(form.Token)
	form.NewPassword = sanitizeInput(form.NewPassword)

	if form.Token == "" {
		common.RespondWithJSON(w, 400, map[string]string{"error": "Reset token is required"})
		return
	}

	if form.NewPassword == "" {
		common.RespondWithJSON(w, 400, map[string]string{"error": "New password is required"})
		return
	}

	// Validate new password complexity
	if err := validatePassword(form.NewPassword); err != nil {
		common.RespondWithJSON(w, 400, map[string]string{"error": err.Error()})
		return
	}

	// Find password reset record by token
	var passwordReset PasswordReset
	err := resetsCollection.FindOne(r.Context(), bson.M{
		"token":      form.Token,
		"used":       false,                     // Token must not be used
		"expires_at": bson.M{"$gt": time.Now()}, // Token must not be expired
	}).Decode(&passwordReset)

	if err != nil {
		if err == mongo.ErrNoDocuments {
			common.RespondWithJSON(w, 400, map[string]string{"error": "Invalid or expired reset token"})
			return
		}
		log.Printf("Failed to find password reset by token: %v", err)
		common.RespondWithJSON(w, 500, map[string]string{"error": "Server error"})
		return
	}

	// Find the user to update
	var user User
	err = usersCollection.FindOne(r.Context(), bson.M{"_id": passwordReset.UserID}).Decode(&user)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			common.RespondWithJSON(w, 400, map[string]string{"error": "Invalid reset token"})
			return
		}
		log.Printf("Failed to find user by ID: %v", err)
		common.RespondWithJSON(w, 500, map[string]string{"error": "Server error"})
		return
	}

	// Hash the new password
	hashedPassword, err := GenerateFromPassword(form.NewPassword, defaultPasswordParams)
	if err != nil {
		log.Printf("Failed to hash new password: %v", err)
		common.RespondWithJSON(w, 500, map[string]string{"error": "Server error"})
		return
	}

	// Update user with new password
	now := time.Now()
	userUpdate := bson.M{
		"$set": bson.M{
			"password":       hashedPassword,
			"updated_at":     now,
			"login_attempts": 0,   // Reset failed login attempts
			"locked_until":   nil, // Unlock account if it was locked
		},
	}

	_, err = usersCollection.UpdateOne(r.Context(), bson.M{"_id": user.ID}, userUpdate)
	if err != nil {
		log.Printf("Failed to update user password: %v", err)
		common.RespondWithJSON(w, 500, map[string]string{"error": "Server error"})
		return
	}

	// Mark password reset token as used
	resetUpdate := bson.M{
		"$set": bson.M{
			"used":    true,
			"used_at": now,
		},
	}

	_, err = resetsCollection.UpdateOne(r.Context(), bson.M{"_id": passwordReset.ID}, resetUpdate)
	if err != nil {
		log.Printf("Failed to mark password reset token as used: %v", err)
		// Don't fail the request, password was already updated
	}

	// Send password change confirmation email (don't fail if this fails)
	if err := SendPasswordChangeConfirmationEmail(user.Email, user.Name); err != nil {
		log.Printf("Failed to send password change confirmation email: %v", err)
		// Continue anyway, password reset was successful
	}

	common.RespondWithJSON(w, 200, map[string]string{
		"message": "Password has been successfully reset. You can now log in with your new password.",
	})
}
