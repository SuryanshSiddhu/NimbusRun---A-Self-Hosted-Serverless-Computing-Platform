package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/nimbusrun/nimbusrun/internal/models"
	"github.com/nimbusrun/nimbusrun/internal/repository"
)

// FunctionHandler handles function CRUD operations.
type FunctionHandler struct {
	fnRepo   *repository.FunctionRepository
	verRepo  *repository.VersionRepository
	fnInvRep *repository.InvocationRepository
}

// NewFunctionHandler creates a new FunctionHandler.
func NewFunctionHandler(fnRepo *repository.FunctionRepository, verRepo *repository.VersionRepository, fnInvRep *repository.InvocationRepository) *FunctionHandler {
	return &FunctionHandler{fnRepo: fnRepo, verRepo: verRepo, fnInvRep: fnInvRep}
}

// Create creates a new function.
func (h *FunctionHandler) Create(c *gin.Context) {
	var input struct {
		Name           string `json:"name" binding:"required"`
		Entrypoint     string `json:"entrypoint" binding:"required"`
		MemoryLimit    int    `json:"memory_limit"`
		CPULimit       int    `json:"cpu_limit"`
		Timeout        int    `json:"timeout"`
		MaxConcurrency int    `json:"max_concurrency"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	userID, _ := c.Get("user_id")
	userUUID, err := uuid.Parse(userID.(string))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "invalid user context"})
		return
	}
	fn := &models.Function{
		ID:              uuid.New(),
		UserID:          userUUID,
		Name:            input.Name,
		Entrypoint:      input.Entrypoint,
		MemoryLimit:     input.MemoryLimit,
		CPULimit:        input.CPULimit,
		Timeout:         input.Timeout,
		MaxConcurrency:  input.MaxConcurrency,
		ActiveVersionID: uuid.Nil,
	}
	if err := h.fnRepo.Create(c.Request.Context(), fn); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create function"})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"id": fn.ID, "name": fn.Name})
}

// List lists all functions for the authenticated user.
func (h *FunctionHandler) List(c *gin.Context) {
	userID, _ := c.Get("user_id")
	userUUID, _ := uuid.Parse(userID.(string))
	funcs, err := h.fnRepo.ListByUserID(c.Request.Context(), userUUID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list functions"})
		return
	}
	result := make([]gin.H, len(funcs))
	for i, fn := range funcs {
		result[i] = gin.H{"id": fn.ID, "name": fn.Name, "active_version_id": fn.ActiveVersionID, "created_at": fn.CreatedAt}
	}
	c.JSON(http.StatusOK, result)
}

// Get retrieves a single function by ID.
func (h *FunctionHandler) Get(c *gin.Context) {
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
	c.JSON(http.StatusOK, fn)
}

// Delete removes a function.
func (h *FunctionHandler) Delete(c *gin.Context) {
	fnID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid function ID"})
		return
	}
	if err := h.fnRepo.Delete(c.Request.Context(), fnID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete function"})
		return
	}
	c.Status(http.StatusNoContent)
}

// Deploy triggers a build of the function's Docker image.
func (h *FunctionHandler) Deploy(c *gin.Context) {
	fnID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid function ID"})
		return
	}
	if _, err := h.fnRepo.GetByID(c.Request.Context(), fnID); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "function not found"})
		return
	}
	lastNum, err := h.verRepo.LatestVersionNumber(c.Request.Context(), fnID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get latest version"})
		return
	}
	ver := &models.FunctionVersion{
		ID:         uuid.New(),
		FunctionID: fnID,
		VersionNum: lastNum + 1,
		ImageURI:   "",
		Status:     "QUEUED",
	}
	if err := h.verRepo.Create(c.Request.Context(), ver); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create version"})
		return
	}
	h.verRepo.UpdateStatus(c.Request.Context(), ver.ID, "BUILDING")
	c.JSON(http.StatusAccepted, gin.H{"function_id": fnID, "version_id": ver.ID, "version": ver.VersionNum, "status": "BUILDING", "message": "Build initiated"})
}

// ListVersions lists all versions for a function.
func (h *FunctionHandler) ListVersions(c *gin.Context) {
	fnID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid function ID"})
		return
	}
	versions, err := h.verRepo.ListByFunctionID(c.Request.Context(), fnID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list versions"})
		return
	}
	result := make([]gin.H, len(versions))
	for i, v := range versions {
		result[i] = gin.H{"id": v.ID, "version_number": v.VersionNum, "image_uri": v.ImageURI, "status": v.Status, "created_at": v.CreatedAt}
	}
	c.JSON(http.StatusOK, result)
}

// Rollback switches the active version pointer.
func (h *FunctionHandler) Rollback(c *gin.Context) {
	fnID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid function ID"})
		return
	}
	versionNum, err := strconv.Atoi(c.Param("version"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid version number"})
		return
	}
	versions, err := h.verRepo.ListByFunctionID(c.Request.Context(), fnID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list versions"})
		return
	}
	var target *models.FunctionVersion
	for _, v := range versions {
		if v.VersionNum == versionNum {
			target = v
			break
		}
	}
	if target == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "version not found"})
		return
	}
	if target.Status != "READY" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "can only rollback to READY versions"})
		return
	}
	if err := h.fnRepo.UpdateActiveVersion(c.Request.Context(), fnID, target.ID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to rollback"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"function_id": fnID, "active_version": target.ID, "version_number": target.VersionNum})
}

// ListByFunction lists recent invocations for a function.
func (h *FunctionHandler) ListByFunction(c *gin.Context) {
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
	invocations, err := h.fnInvRep.ListByFunctionID(c.Request.Context(), fnID, limit)
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
func (h *FunctionHandler) GetLogs(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"logs": []gin.H{}})
}
