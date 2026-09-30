package token

import (
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

var (
	ErrInvalidToken = errors.New("invalid or expired JWT token")
	ErrTokenExpired = errors.New("token has expired")
)

// Claims represents the JWT claims payload containing tenant and user metadata.
type Claims struct {
	UserID   string `json:"user_id"`
	Username string `json:"username"`
	jwt.RegisteredClaims
}

// Manager manages signing and verification of JWT authentication tokens.
type Manager struct {
	secretKey     []byte
	tokenDuration time.Duration
}

// NewManager creates a new JWT token manager.
func NewManager(secretKey string, duration time.Duration) *Manager {
	if duration <= 0 {
		duration = 24 * time.Hour
	}
	return &Manager{
		secretKey:     []byte(secretKey),
		tokenDuration: duration,
	}
}

// GenerateToken creates a signed JWT string for a validated user identity.
func (m *Manager) GenerateToken(userID, username string) (string, error) {
	claims := Claims{
		UserID:   userID,
		Username: username,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(m.tokenDuration)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			NotBefore: jwt.NewNumericDate(time.Now()),
			Issuer:    "accounting-microservices/auth",
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(m.secretKey)
}

// ValidateToken parses and verifies the validity of a JWT string.
func (m *Manager) ValidateToken(tokenString string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return m.secretKey, nil
	})

	if err != nil {
		return nil, ErrInvalidToken
	}

	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, ErrInvalidToken
	}

	return claims, nil
}
