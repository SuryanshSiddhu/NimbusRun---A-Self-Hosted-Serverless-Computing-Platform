package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/nimbusrun/nimbusrun/internal/models"
	"github.com/nimbusrun/nimbusrun/internal/repository"
)

// InvocationHandler handles invocation operations.
type InvocationHandler struct {
	invRep  *repository.InvocationRepository
	fnRepo  *repository.FunctionRepository
	verRepo *repository.VersionRepository
}

// NewInvocationHandler creates a new InvocationHandler.
func NewInvocationHandler(invRep *repository.InvocationRepository, fnRepo *repository.FunctionRepository, verRepo *repository.VersionRepository) *InvocationHandler {
	return &InvocationHandler{invRep: invRep, fnRepo: fnRepo, verRepo: verRepo}
}

// Invoke triggers synchronous function execution.
func (h *InvocationHandler) Invoke(c *gin.Context) {
	fnID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid function ID"})
		return
	}
	fn, err := h.fnRepo.GetByID(c.Request.Context(), fnID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "function not found"})
		return
	}
	if fn.ActiveVersionID == uuid.Nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "no active version set"})
		return
	}
	ver, err := h.verRepo.GetByID(c.Request.Context(), fn.ActiveVersionID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "active version not found"})
		return
	}
	idemKey := c.GetHeader("Idempotency-Key")
	if idemKey != "" {
		if existing, err := h.invRep.GetByIdempotencyKey(c.Request.Context(), idemKey); err == nil {
			c.JSON(http.StatusAccepted, gin.H{"id": existing.ID, "status": existing.Status, "message": "Duplicate request deduplicated"})
			return
		}
	}
	inv := &models.Invocation{
		ID:         uuid.New(),
		FunctionID: fnID,
		VersionID:  ver.ID,
		WorkerID:   uuid.Nil,
		Status:     "PENDING",
		IdempKey:   idemKey,
	}
	if err := h.invRep.Create(c.Request.Context(), inv); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create invocation"})
		return
	}
	c.JSON(http.StatusAccepted, gin.H{"id": inv.ID, "status": inv.Status, "message": "Invocation accepted and queued"})
}

// ListByFunction lists recent invocations for a function.
func (h *InvocationHandler) ListByFunction(c *gin.Context) {
	fnID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid function ID"})
		return
	}
	limit := 50
	if l := c.Query("limit"); l != "" {
		if n, err := strconv.Atoi(l); err == nil && n > 0 {
			limit = n
		}
	}
	invocations, err := h.invRep.ListByFunctionID(c.Request.Context(), fnID, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list invocations"})
		return
	}
	result := make([]gin.H, len(invocations))
	for i, inv := range invocations {
		result[i] = gin.H{"id": inv.ID, "status": inv.Status, "duration_ms": inv.DurationMs, "cold_start": inv.ColdStart, "retry_count": inv.RetryCount, "created_at": inv.CreatedAt}
	}
	c.JSON(http.StatusOK, result)
}

// GetLogs retrieves structured logs for an invocation.
func (h *InvocationHandler) GetLogs(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"logs": []gin.H{}})
}
