package isolation

import (
	"context"
	"fmt"
	"time"

	"github.com/moby/moby/api/types/container"
)

// ResourceLimits defines the resource constraints for a container.
type ResourceLimits struct {
	MemoryMB           int
	CPUMillicores      int
	TimeoutSeconds     int
	MaxPIDs            int
	NetworkEnabled     bool
	ReadOnlyFilesystem bool
	NonRoot            bool
	ScratchSizeMB      int
}

// DefaultLimits returns conservative defaults from the PRD.
func DefaultLimits() ResourceLimits {
	return ResourceLimits{
		MemoryMB:           128,
		CPUMillicores:      100,
		TimeoutSeconds:     30,
		MaxPIDs:            256,
		NetworkEnabled:     false,
		ReadOnlyFilesystem: true,
		NonRoot:            true,
		ScratchSizeMB:      64,
	}
}

// ApplyToHostConfig converts the limits to a Docker HostConfig.
func (r ResourceLimits) ApplyToHostConfig() *container.HostConfig {
	memBytes := int64(r.MemoryMB) * 1024 * 1024
	nanoCPUs := int64(r.CPUMillicores) * 1e6

	hc := &container.HostConfig{
		Memory:    memBytes,
		NanoCPUs:  nanoCPUs,
		Resources: container.Resources{
			Memory:   memBytes,
			NanoCPUs: nanoCPUs,
		},
		PidsLimit:      &r.MaxPIDs,
		ReadonlyRootfs: r.ReadOnlyFilesystem,
	}

	if !r.NetworkEnabled {
		hc.NetworkMode = "none"
	}

	return hc
}

// SecurityConfig builds security options: cap drop, user, etc.
func (r ResourceLimits) SecurityConfig() (capDrop []string, user string) {
	capDrop = []string{"ALL"}
	if r.NonRoot {
		// Use UID 1000 (non-root)
		user = "1000:1000"
	}
	return capDrop, user
}

// Validate ensures the resource limits are within safe bounds.
func (r ResourceLimits) Validate() error {
	if r.MemoryMB < 32 || r.MemoryMB > 4096 {
		return fmt.Errorf("memory limit %dMB out of range [32, 4096]", r.MemoryMB)
	}
	if r.CPUMillicores < 50 || r.CPUMillicores > 2000 {
		return fmt.Errorf("CPU limit %dm out of range [50, 2000]", r.CPUMillicores)
	}
	if r.TimeoutSeconds < 1 || r.TimeoutSeconds > 300 {
		return fmt.Errorf("timeout %ds out of range [1, 300]", r.TimeoutSeconds)
	}
	if r.MaxPIDs < 16 || r.MaxPIDs > 1024 {
		return fmt.Errorf("PID limit %d out of range [16, 1024]", r.MaxPIDs)
	}
	return nil
}

// FormatMemory returns a human-readable memory size.
func (r ResourceLimits) FormatMemory() string {
	return fmt.Sprintf("%dMB", r.MemoryMB)
}

// Timeout returns the time.Duration for the configured timeout.
func (r ResourceLimits) Timeout() time.Duration {
	return time.Duration(r.TimeoutSeconds) * time.Second
}

// ContextWithTimeout creates a context that cancels at the timeout.
func (r ResourceLimits) ContextWithTimeout(ctx context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(ctx, r.Timeout())
}
