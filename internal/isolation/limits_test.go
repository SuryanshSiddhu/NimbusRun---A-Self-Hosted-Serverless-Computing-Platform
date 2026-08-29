package isolation

import (
	"testing"
)

func TestDefaultLimits(t *testing.T) {
	limits := DefaultLimits()
	if err := limits.Validate(); err != nil {
		t.Errorf("Default limits failed validation: %v", err)
	}
}

func TestValidate(t *testing.T) {
	tests := []struct {
		name    string
		limits  ResourceLimits
		wantErr bool
	}{
		{
			name:    "valid",
			limits:  DefaultLimits(),
			wantErr: false,
		},
		{
			name: "memory too low",
			limits: ResourceLimits{
				MemoryMB:       8,
				CPUMillicores:  100,
				TimeoutSeconds: 30,
				MaxPIDs:        256,
			},
			wantErr: true,
		},
		{
			name: "memory too high",
			limits: ResourceLimits{
				MemoryMB:       8192,
				CPUMillicores:  100,
				TimeoutSeconds: 30,
				MaxPIDs:        256,
			},
			wantErr: true,
		},
		{
			name: "CPU too low",
			limits: ResourceLimits{
				MemoryMB:       128,
				CPUMillicores:  10,
				TimeoutSeconds: 30,
				MaxPIDs:        256,
			},
			wantErr: true,
		},
		{
			name: "timeout too long",
			limits: ResourceLimits{
				MemoryMB:       128,
				CPUMillicores:  100,
				TimeoutSeconds: 600,
				MaxPIDs:        256,
			},
			wantErr: true,
		},
		{
			name: "PIDs too low",
			limits: ResourceLimits{
				MemoryMB:       128,
				CPUMillicores:  100,
				TimeoutSeconds: 30,
				MaxPIDs:        8,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.limits.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestSecurityConfig(t *testing.T) {
	limits := DefaultLimits()
	capDrop, user := limits.SecurityConfig()
	if len(capDrop) == 0 {
		t.Error("SecurityConfig should drop all capabilities")
	}
	if user == "" {
		t.Error("SecurityConfig should set non-root user")
	}
}

func TestApplyToHostConfig(t *testing.T) {
	limits := ResourceLimits{
		MemoryMB:           256,
		CPUMillicores:      500,
		TimeoutSeconds:     60,
		MaxPIDs:            128,
		NetworkEnabled:     false,
		ReadOnlyFilesystem: true,
		NonRoot:            true,
	}

	hc := limits.ApplyToHostConfig()
	if hc.Memory != int64(256*1024*1024) {
		t.Errorf("Memory = %d, want %d", hc.Memory, 256*1024*1024)
	}
	if hc.NanoCPUs != int64(500*1e6) {
		t.Errorf("NanoCPUs = %d, want %d", hc.NanoCPUs, 500*1e6)
	}
	if !hc.ReadonlyRootfs {
		t.Error("ReadonlyRootfs should be true")
	}
	if hc.NetworkMode != "none" {
		t.Errorf("NetworkMode = %s, want 'none'", hc.NetworkMode)
	}
}
