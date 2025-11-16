package common

import (
	"context"
	"log"
	"net/http"
	"time"

	"github.com/google/uuid"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

type VerifyEmailForm struct {
	Token string `json:"token" binding:"required"` // The verification token
}

type ResendVerificationEmailForm struct {
	Email string `json:"email" binding:"required"` // The email of the user
}

// EmailVerification represents an email verification request in the database
type EmailVerification struct {
	ID        string     `json:"id" bson:"_id"`                // Unique ID for the verification request
	Name      string     `json:"name" bson:"name"`             // Name of the user requesting verification
	UserID    string     `json:"user_id" bson:"user_id"`       // ID of the user requesting verification
	Email     string     `json:"email" bson:"email"`           // Email of the user (for easier queries)
	Token     string     `json:"token" bson:"token"`           // The verification token
	ExpiresAt time.Time  `json:"expires_at" bson:"expires_at"` // When the token expires
	CreatedAt time.Time  `json:"created_at" bson:"created_at"` // When the verification was requested
	Used      bool       `json:"used" bson:"used"`             // Whether the token has been used
	UsedAt    *time.Time `json:"used_at" bson:"used_at"`       // When the token was used
}

// CreateEmailVerification creates a new email verification record
func CreateEmailVerification(database *mongo.Database, userID, email, token string) error {
	verificationsCollection := database.Collection("email_verifications")

	// Generate unique ID for the verification request
	verificationID, err := uuid.NewV7()
	if err != nil {
		return err
	}

	// Create email verification record
	now := time.Now()
	emailVerification := EmailVerification{
		ID:        verificationID.String(),
		UserID:    userID,
		Email:     email,
		Token:     token,
		ExpiresAt: now.Add(24 * time.Hour), // Token expires in 24 hours
		CreatedAt: now,
		Used:      false,
		UsedAt:    nil,
	}

	// Insert the verification record
	_, err = verificationsCollection.InsertOne(context.TODO(), emailVerification)
	return err
}

// VerifyEmail handles email verification
func VerifyEmail(database *mongo.Database, w http.ResponseWriter, r *http.Request) {
	usersCollection := database.Collection("users")
	verificationsCollection := database.Collection("email_verifications")

	var form VerifyEmailForm
	if !ValidateAndBindJSON(w, r, &form) {
		return
	}

	// Sanitize token input
	form.Token = sanitizeInput(form.Token)

	if form.Token == "" {
		RespondWithJSON(w, 400, map[string]string{"error": "Verification token is required"})
		return
	}

	// Validate token format (should be exactly 8 digits)
	if err := validateVerificationToken(form.Token); err != nil {
		RespondWithJSON(w, 400, map[string]string{"error": err.Error()})
		return
	}

	// Find verification record by token
	var verification EmailVerification
	err := verificationsCollection.FindOne(r.Context(), bson.M{
		"token":      form.Token,
		"used":       false,                     // Token must not be used
		"expires_at": bson.M{"$gt": time.Now()}, // Token must not be expired
	}).Decode(&verification)

	if err != nil {
		if err == mongo.ErrNoDocuments {
			RespondWithJSON(w, 400, map[string]string{"error": "Invalid or expired verification token"})
			return
		}
		log.Printf("Failed to find verification by token: %v", err)
		RespondWithJSON(w, 500, map[string]string{"error": "Server error"})
		return
	}

	// Find the user to verify
	var user User
	err = usersCollection.FindOne(r.Context(), bson.M{
		"_id":         verification.UserID,
		"is_verified": false, // Only allow verification of unverified accounts
	}).Decode(&user)

	if err != nil {
		if err == mongo.ErrNoDocuments {
			RespondWithJSON(w, 400, map[string]string{"error": "Invalid verification token or account already verified"})
			return
		}
		log.Printf("Failed to find user by ID: %v", err)
		RespondWithJSON(w, 500, map[string]string{"error": "Server error"})
		return
	}

	// Update user to mark as verified
	now := time.Now()
	userUpdate := bson.M{
		"$set": bson.M{
			"is_verified": true,
			"verified_at": now,
			"updated_at":  now,
		},
	}

	_, err = usersCollection.UpdateOne(r.Context(), bson.M{"_id": user.ID}, userUpdate)
	if err != nil {
		log.Printf("Failed to update user verification status: %v", err)
		RespondWithJSON(w, 500, map[string]string{"error": "Server error"})
		return
	}

	// Mark verification token as used
	verificationUpdate := bson.M{
		"$set": bson.M{
			"used":    true,
			"used_at": now,
		},
	}

	_, err = verificationsCollection.UpdateOne(r.Context(), bson.M{"_id": verification.ID}, verificationUpdate)
	if err != nil {
		log.Printf("Failed to mark verification token as used: %v", err)
		// Don't fail the request, user is already verified
	}

	// Send welcome email (don't fail if this fails)
	if err := SendWelcomeEmail(user.Email, user.Name); err != nil {
		log.Printf("Failed to send welcome email: %v", err)
		// Continue anyway, verification was successful
	}

	RespondWithJSON(w, 200, map[string]interface{}{
		"message": "Email verified successfully! You can now log in.",
		"user": map[string]string{
			"id":    user.ID,
			"email": user.Email,
			"name":  user.Name,
		},
	})
}


func ResendVerificationEmail(database *mongo.Database, w http.ResponseWriter, r *http.Request) {
	var form ResendVerificationEmailForm
	if !ValidateAndBindJSON(w, r, &form) {
		return
	}

	collection := database.Collection("email_verifications")

	var emailVerification EmailVerification
	err := collection.FindOne(r.Context(), bson.M{"email": form.Email}).Decode(&emailVerification)
	if err != nil {
		RespondWithJSON(w, 400, map[string]string{"error": "Email verification not found"})
		return
	}

	// Send verification email
	if err := SendVerificationEmail(emailVerification.Email, emailVerification.Name, "templates/verify.html", emailVerification.Token); err != nil {
		log.Printf("Failed to send verification email: %v", err)
		// Don't fail the registration if email sending fails
		// The user is still created and can request a new verification email
	}

	RespondWithJSON(w, 200, map[string]string{
		"message": "Registration successful. Please check your email to verify your account.",
		"email":   emailVerification.Email,
	})
}
