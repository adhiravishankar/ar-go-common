package common

import (
	"context"
	"net/http"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

// contextKey is a custom type for context keys to avoid collisions
type contextKey string

const (
	userKey contextKey = "userID"
)

// setUserID stores the user ID in the request context
func setUserID(r *http.Request, userID string) *http.Request {
	ctx := context.WithValue(r.Context(), userKey, userID)
	return r.WithContext(ctx)
}

// getUserID retrieves the user ID from the request context
func getUserID(r *http.Request) string {
	userID, _ := r.Context().Value(userKey).(string)
	return userID
}

// User has better field ordering for memory efficiency
type User struct {
	// time.Time fields first (largest)
	CreatedAt   time.Time  `json:"created_at" bson:"created_at"`
	UpdatedAt   time.Time  `json:"-" bson:"updated_at"`
	LastLoginAt time.Time  `json:"-" bson:"last_login_at"`
	VerifiedAt  *time.Time `json:"-" bson:"verified_at"`  // 8 bytes (pointer)
	LockedUntil *time.Time `json:"-" bson:"locked_until"` // 8 bytes (pointer)

	// String fields
	ID       string `json:"id" bson:"_id"`
	Email    string `json:"email" bson:"email"`
	Password string `json:"-" bson:"password"`
	Name     string `json:"name" bson:"name"`

	// Smaller integer and boolean fields grouped together
	LoginAttempts int  `json:"-" bson:"login_attempts"` // 8 bytes on 64-bit
	IsVerified    bool `json:"-" bson:"is_verified"`    // 1 byte
}

func GetUser(database *mongo.Database, w http.ResponseWriter, r *http.Request) {
	userID := getUserID(r)
	if userID == "" {
		RespondWithJSON(w, http.StatusUnauthorized, map[string]string{"error": "Unauthorized"})
		return
	}

	var user User
	err := database.Collection("users").FindOne(r.Context(), bson.M{"_id": userID}).Decode(&user)
	if err != nil {
		RespondWithJSON(w, http.StatusInternalServerError, map[string]string{"error": "Failed to get user"})
		return
	}

	RespondWithJSON(w, http.StatusOK, user)
}

func UpdateUser(database *mongo.Database, w http.ResponseWriter, r *http.Request) {
	userID := getUserID(r)
	if userID == "" {
		RespondWithJSON(w, http.StatusUnauthorized, map[string]string{"error": "Unauthorized"})
		return
	}

	userForm := User{}
	if !ValidateAndBindJSON(w, r, &userForm) {
		return
	}
	userForm.UpdatedAt = time.Now()

	_, err := database.Collection("users").UpdateOne(r.Context(), bson.M{"_id": userID}, bson.M{"$set": userForm})
	if err != nil {
		RespondWithJSON(w, http.StatusInternalServerError, map[string]string{"error": "Failed to update user"})
		return
	}

	RespondWithJSON(w, http.StatusOK, userForm)
}
