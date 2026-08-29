package auth

import (
	"crypto/rand"
	"crypto/subtle"
	"errors"
	"fmt"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

const apiKeyPrefix = "nimbus_"

// hasher interface allows injection in tests.
type hasher interface {
	Hash(password string) (string, error)
	Compare(hashedPassword, password string) error
}

type bcryptHasher struct{}

func (b bcryptHasher) Hash(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	return string(hash), err
}

func (b bcryptHasher) Compare(hashedPassword, password string) error {
	return bcrypt.CompareHashAndPassword([]byte(hashedPassword), []byte(password))
}

// Service provides authentication and token issuance.
type Service struct {
	jwtSecret []byte
	hasher    hasher
	tokenTTL  time.Duration
}

// NewService creates a new auth Service.
func NewService(jwtSecret string, tokenTTL time.Duration) *Service {
	return &Service{
		jwtSecret: []byte(jwtSecret),
		hasher:    bcryptHasher{},
		tokenTTL:  tokenTTL,
	}
}

// JWTSecret returns the raw JWT secret (for middleware token verification).
func (s *Service) JWTSecret() []byte {
	return s.jwtSecret
}

// APIKeyHeader returns the API key header name (defaults to "X-API-Key").
func (s *Service) APIKeyHeader() string {
	return "X-API-Key"
}

// TokenTTL returns the token TTL.
func (s *Service) TokenTTL() time.Duration {
	return s.tokenTTL
}

// ValidateToken parses and validates a JWT, returning the user ID and token type.
func (s *Service) ValidateToken(tokenStr string) (uuid.UUID, string, error) {
	token, err := jwt.Parse(tokenStr, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return s.jwtSecret, nil
	})
	if err != nil {
		return uuid.Nil, "", err
	}
	if !token.Valid {
		return uuid.Nil, "", fmt.Errorf("invalid token")
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return uuid.Nil, "", fmt.Errorf("invalid claims")
	}

	sub, _ := claims["sub"].(string)
	uid, err := uuid.Parse(sub)
	if err != nil {
		return uuid.Nil, "", fmt.Errorf("invalid subject: %w", err)
	}

	typ, _ := claims["type"].(string)
	return uid, typ, nil
}

// HashPassword returns a bcrypt hash of the given password.
func HashPassword(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	return string(hash), err
}

// GenerateAPIKey creates a new random API key.
func GenerateAPIKey() (string, error) {
	b := make([]byte, 24)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return apiKeyPrefix + fmt.Sprintf("%x", b), nil
}

// ValidateAPIKey returns true if key matches prefix + expected.
func ValidateAPIKey(given string, expected string) bool {
	if !strings.HasPrefix(given, apiKeyPrefix) {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(given), []byte(expected)) == 1
}

// IssueTokens returns an access token and refresh token for the user.
func (s *Service) IssueTokens(userID uuid.UUID) (accessToken string, refreshToken string, err error) {
	accessClaims := jwt.MapClaims{
		"sub": userID.String(),
		"exp": time.Now().Add(s.tokenTTL).Unix(),
		"iat": time.Now().Unix(),
		"type": "access",
	}
	accessToken, err = jwt.NewWithClaims(jwt.SigningMethodHS256, accessClaims).SignedString(s.jwtSecret)
	if err != nil {
		return "", "", fmt.Errorf("signing access token: %w", err)
	}

	refreshClaims := jwt.MapClaims{
		"sub": userID.String(),
		"exp": time.Now().Add(7 * 24 * time.Hour).Unix(),
		"iat": time.Now().Unix(),
		"type": "refresh",
	}
	refreshToken, err = jwt.NewWithClaims(jwt.SigningMethodHS256, refreshClaims).SignedString(s.jwtSecret)
	if err != nil {
		return "", "", fmt.Errorf("signing refresh token: %w", err)
	}

	return accessToken, refreshToken, nil
}

// ValidateAccessToken returns the user ID if the access token is valid.
func (s *Service) ValidateAccessToken(tokenStr string) (uuid.UUID, error) {
	token, err := jwt.Parse(tokenStr, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return s.jwtSecret, nil
	})
	if err != nil {
		return uuid.Nil, err
	}
	if claims, ok := token.Claims.(jwt.MapClaims); ok && token.Valid {
		typ, _ := claims["type"].(string)
		if typ != "access" {
			return uuid.Nil, errors.New("token is not an access token")
		}
		sub, _ := claims["sub"].(string)
		return uuid.Parse(sub)
	}
	return uuid.Nil, errors.New("invalid token")
}

// VerifyPassword compares a plaintext password with its bcrypt hash.
func VerifyPassword(hash, password string) error {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
}
