package common

import (
	"context"
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"golang.org/x/crypto/argon2"
)

type LoginForm struct {
	Email    string `json:"email" binding:"required"`    // The email of the user
	Password string `json:"password" binding:"required"` // The password of the user
}

func Login(database *mongo.Database, w http.ResponseWriter, r *http.Request) {
	collection := database.Collection("users")

	// Get the request body
	var form LoginForm
	if !ValidateAndBindJSON(w, r, &form) {
		return
	}

	// Sanitize username
	form.Email = SanitizeInput(form.Email)

	// Find the user in the database
	var user User
	err := collection.FindOne(r.Context(), bson.M{"email": form.Email}).Decode(&user)
	if err != nil {
		// Use generic error message to prevent user enumeration
		RespondWithJSON(w, 401, map[string]string{"error": "Invalid credentials"})
		return
	}

	// Check if account is locked
	if user.LockedUntil != nil && time.Now().Before(*user.LockedUntil) {
		RespondWithJSON(w, 423, map[string]string{"error": "Account temporarily locked"})
		return
	}

	// Check if the password matches
	match, err := ComparePasswordAndHash(form.Password, user.Password)
	if err != nil {
		log.Printf("Password comparison error for user %s: %v", user.Email, err)
		RespondWithJSON(w, 401, map[string]string{"error": "Invalid credentials"})
		return
	}

	if !match {
		// Increment failed login attempts
		user.LoginAttempts++

		// Lock account after 5 failed attempts for 15 minutes
		if user.LoginAttempts >= 5 {
			lockUntil := time.Now().Add(15 * time.Minute)
			user.LockedUntil = &lockUntil
		}

		// Update user record
		collection.UpdateOne(r.Context(), bson.M{"_id": user.ID}, bson.M{
			"$set": bson.M{
				"login_attempts": user.LoginAttempts,
				"locked_until":   user.LockedUntil,
			},
		})

		RespondWithJSON(w, 401, map[string]string{"error": "Invalid credentials"})
		return
	}

	// Check if email is verified
	if !user.IsVerified {
		RespondWithJSON(w, 403, map[string]interface{}{
			"error": "Please verify your email address before logging in. Check your email for a verification link.",
			"email": user.Email,
		})
		return
	}

	// Reset login attempts on successful login
	user.LoginAttempts = 0
	user.LockedUntil = nil
	user.LastLoginAt = time.Now()

	// Generate new token (don't store in database)
	token := jwt.NewWithClaims(jwt.SigningMethodHS512, jwt.MapClaims{
		"iat": time.Now().Unix(),
		"sub": user.ID,
		"exp": time.Now().Add(time.Hour * 24).Unix(),
		"jti": uuid.New().String(),
		"iss": "flight-history-app",
		"aud": "flight-history-users",
	})

	tokenString, err := token.SignedString([]byte(os.Getenv("JWT_SECRET")))
	if err != nil {
		log.Printf("Failed to sign JWT: %v", err)
		RespondWithJSON(w, 500, map[string]string{"error": "Server error"})
		return
	}

	// Update user record
	collection.UpdateOne(r.Context(), bson.M{"_id": user.ID}, bson.M{
		"$set": bson.M{
			"login_attempts": user.LoginAttempts,
			"locked_until":   user.LockedUntil,
			"last_login_at":  user.LastLoginAt,
		},
	})

	// Upgrade password hash if needed
	go RehashPasswordIfNeeded(database, form.Password, &user)

	RespondWithJSON(w, 200, map[string]interface{}{
		"token": tokenString,
		"user": map[string]string{
			"id":    user.ID,
			"email": user.Email,
			"name":  user.Name,
		},
	})
}

// rehashPasswordIfNeeded checks if the user's password hash uses the latest
// recommended parameters, and if not, re-hashes it and updates it in the database.
// This is done in a goroutine to not block the login request.
// RehashPasswordIfNeeded checks if the user's password hash uses the latest
// recommended parameters, and if not, re-hashes it and updates it in the database.
// This is done in a goroutine to not block the login request.
func RehashPasswordIfNeeded(database *mongo.Database, password string, user *User) {
	p, _, _, err := DecodeHash(user.Password)
	if err != nil {
		log.Printf("rehash: could not decode password hash for user %s: %v\n", user.Email, err)
		return
	}

	// For now, we only check the parallelism parameter.
	if p.parallelism != defaultPasswordParams.parallelism || p.memory != defaultPasswordParams.memory || p.iterations != defaultPasswordParams.iterations {
		log.Printf("rehash: parameters for user %s are outdated, re-hashing password\n", user.Email)

		hashedPassword, err := GenerateFromPassword(password, defaultPasswordParams)
		if err != nil {
			log.Printf("rehash: error re-hashing password for user %s: %v\n", user.Email, err)
			return
		}

		collection := database.Collection("users")
		_, err = collection.UpdateOne(context.Background(), bson.M{"_id": user.ID}, bson.M{"$set": bson.M{"password": hashedPassword}})
		if err != nil {
			log.Printf("rehash: error updating password for user %s: %v\n", user.Email, err)
		}
	}
}

func ComparePasswordAndHash(password string, encodedHash string) (match bool, err error) {
	// Extract the parameters, salt and derived key from the encoded password
	// hash.
	p, salt, hash, err := DecodeHash(encodedHash)
	if err != nil {
		return false, err
	}

	// Derive the key from the other password using the same parameters.
	otherHash := argon2.IDKey([]byte(password), salt, p.iterations, p.memory, p.parallelism, p.keyLength)

	// Check that the contents of the hashed passwords are identical. Note
	// that we are using the subtle.ConstantTimeCompare() function for this
	// to help prevent timing attacks.
	if subtle.ConstantTimeCompare(hash, otherHash) == 1 {
		return true, nil
	}
	return false, nil
}

func DecodeHash(encodedHash string) (p *PasswordParams, salt, hash []byte, err error) {
	vals := strings.Split(encodedHash, "$")
	if len(vals) != 6 {
		return nil, nil, nil, ErrInvalidHash
	}

	var version int
	_, err = fmt.Sscanf(vals[2], "v=%d", &version)
	if err != nil {
		return nil, nil, nil, err
	}
	if version != argon2.Version {
		return nil, nil, nil, ErrIncompatibleVersion
	}

	p = &PasswordParams{}
	_, err = fmt.Sscanf(vals[3], "m=%d,t=%d,p=%d", &p.memory, &p.iterations, &p.parallelism)
	if err != nil {
		return nil, nil, nil, err
	}

	salt, err = base64.RawStdEncoding.Strict().DecodeString(vals[4])
	if err != nil {
		return nil, nil, nil, err
	}
	p.saltLength = uint32(len(salt))

	hash, err = base64.RawStdEncoding.Strict().DecodeString(vals[5])
	if err != nil {
		return nil, nil, nil, err
	}
	p.keyLength = uint32(len(hash))

	return p, salt, hash, nil
}
