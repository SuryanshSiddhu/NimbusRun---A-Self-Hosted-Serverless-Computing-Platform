package handler

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/nimbusrun/nimbusrun/internal/auth"
	"github.com/nimbusrun/nimbusrun/internal/db"
)

// AuthHandler handles authentication endpoints.
type AuthHandler struct {
	db     *db.DB
	svc    *auth.Service
	jwtTTL time.Duration
}

// NewAuthHandler creates a new AuthHandler.
func NewAuthHandler(database *db.DB, svc *auth.Service) *AuthHandler {
	return &AuthHandler{
		db:     database,
		svc:    svc,
		jwtTTL: 24 * time.Hour,
	}
}

// Register handles new user registration.
func (h *AuthHandler) Register(c *gin.Context) {
	var input struct {
		Email    string `json:"email" binding:"required,email"`
		Password string `json:"password" binding:"required,min=6"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Hash password
	hashedPwd, err := auth.HashPassword(input.Password)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to hash password"})
		return
	}

	// Create user
	apiKey, err := auth.GenerateAPIKey()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to generate API key"})
		return
	}

	userID := uuid.New()
	_, err = h.db.Pool.Exec(c.Request.Context(), `
		INSERT INTO users (id, email, password_hash, api_key)
		VALUES ($1, $2, $3, $4)
	`, userID, input.Email, hashedPwd, apiKey)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create user"})
		return
	}

	// Issue tokens
	accessToken, refreshToken, err := h.svc.IssueTokens(userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to issue tokens"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"user_id":       userID,
		"email":         input.Email,
		"access_token":  accessToken,
		"refresh_token": refreshToken,
		"api_key":       apiKey,
		"token_type":    "bearer",
		"expires_in":    int(h.jwtTTL.Seconds()),
	})
}

// Login handles user login.
func (h *AuthHandler) Login(c *gin.Context) {
	var input struct {
		Email    string `json:"email" binding:"required,email"`
		Password string `json:"password" binding:"required,min=6"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Find user
	var userID uuid.UUID
	var email, passwordHash, apiKey string
	err := h.db.Pool.QueryRow(c.Request.Context(), `
		SELECT id, email, password_hash, api_key FROM users WHERE email = $1
	`, input.Email).Scan(&userID, &email, &passwordHash, &apiKey)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid credentials"})
		return
	}

	// Verify password
	if err := auth.VerifyPassword(passwordHash, input.Password); err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid credentials"})
		return
	}

	// Issue tokens
	accessToken, refreshToken, err := h.svc.IssueTokens(userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to issue tokens"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"user_id":       userID,
		"email":         email,
		"access_token":  accessToken,
		"refresh_token": refreshToken,
		"api_key":       apiKey,
		"token_type":    "bearer",
		"expires_in":    int(h.jwtTTL.Seconds()),
	})
}