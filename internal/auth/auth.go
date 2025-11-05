// internal/auth/auth.go
package auth

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"strings"
	"time"
)

// TokenConfig holds the configuration for token generation
type TokenConfig struct {
	Secret     []byte
	Expiration time.Duration
}

// Token represents an authentication token
type Token struct {
	UserID    string            `json:"user_id"`
	ExpiresAt int64             `json:"expires_at"`
	IssuedAt  int64             `json:"issued_at"`
	Claims    map[string]string `json:"claims,omitempty"`
}

// GenerateToken creates a new authentication token
func GenerateToken(userID string, config *TokenConfig) (string, error) {
	if len(config.Secret) == 0 {
		return "", fmt.Errorf("secret key is required")
	}

	token := &Token{
		UserID:    userID,
		ExpiresAt: time.Now().Add(config.Expiration).Unix(),
		IssuedAt:  time.Now().Unix(),
		Claims:    make(map[string]string),
	}

	// Create the token payload
	payload := fmt.Sprintf("%s|%d|%d", token.UserID, token.ExpiresAt, token.IssuedAt)

	// Create HMAC signature
	h := hmac.New(sha256.New, config.Secret)
	h.Write([]byte(payload))
	signature := h.Sum(nil)

	// Encode payload and signature
	encodedPayload := base64.URLEncoding.EncodeToString([]byte(payload))
	encodedSignature := base64.URLEncoding.EncodeToString(signature)

	// Combine payload and signature
	tokenString := fmt.Sprintf("%s.%s", encodedPayload, encodedSignature)

	return tokenString, nil
}

// ParseToken parses and validates a token
func ParseToken(tokenString string, config *TokenConfig) (*Token, error) {
	if len(config.Secret) == 0 {
		return nil, fmt.Errorf("secret key is required")
	}

	parts := strings.Split(tokenString, ".")
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid token format")
	}

	encodedPayload := parts[0]
	encodedSignature := parts[1]

	// Decode payload and signature
	payloadBytes, err := base64.URLEncoding.DecodeString(encodedPayload)
	if err != nil {
		return nil, fmt.Errorf("invalid token payload: %w", err)
	}

	signatureBytes, err := base64.URLEncoding.DecodeString(encodedSignature)
	if err != nil {
		return nil, fmt.Errorf("invalid token signature: %w", err)
	}

	// Verify signature
	expectedSignature := hmac.New(sha256.New, config.Secret)
	expectedSignature.Write(payloadBytes)
	expectedSignatureBytes := expectedSignature.Sum(nil)

	if !hmac.Equal(signatureBytes, expectedSignatureBytes) {
		return nil, fmt.Errorf("invalid token signature")
	}

	// Parse payload
	payload := string(payloadBytes)
	payloadParts := strings.Split(payload, "|")
	if len(payloadParts) != 3 {
		return nil, fmt.Errorf("invalid payload format")
	}

	userID := payloadParts[0]
	expiresAt := parseTimestamp(payloadParts[1])
	issuedAt := parseTimestamp(payloadParts[2])

	// Check expiration
	if time.Now().Unix() > expiresAt {
		return nil, fmt.Errorf("token has expired")
	}

	return &Token{
		UserID:    userID,
		ExpiresAt: expiresAt,
		IssuedAt:  issuedAt,
	}, nil
}

// parseTimestamp converts string timestamp to int64
func parseTimestamp(timestampStr string) int64 {
	var timestamp int64
	fmt.Sscanf(timestampStr, "%d", &timestamp)
	return timestamp
}

// GenerateSecureKey generates a secure random key for token signing
func GenerateSecureKey(length int) ([]byte, error) {
	if length <= 0 {
		length = 32 // Default to 256 bits
	}

	key := make([]byte, length)
	_, err := rand.Read(key)
	if err != nil {
		return nil, err
	}

	return key, nil
}
