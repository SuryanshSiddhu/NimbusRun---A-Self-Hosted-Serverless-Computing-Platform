package worker

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/api/types/mount"
	"github.com/moby/moby/client"
)

// Runner executes user functions inside Docker containers.
type Runner struct {
	cli     *client.Client
	timeout time.Duration
}

// ContainerResult holds the outcome of a container run.
type ContainerResult struct {
	Stdout   string
	Stderr   string
	ExitCode int
	Duration time.Duration
	Err      error
}

// NewRunner creates a new function Runner.
func NewRunner(timeout time.Duration) (*Runner, error) {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, fmt.Errorf("creating docker client: %w", err)
	}
	return &Runner{cli: cli, timeout: timeout}, nil
}

// Run executes a function inside an isolated Docker container.
func (r *Runner) Run(ctx context.Context, req *RunRequest) *ContainerResult {
	start := time.Now()

	if err := r.pullImage(ctx, req.ImageURI); err != nil {
		return &ContainerResult{Err: fmt.Errorf("pulling image: %w", err)}
	}

	payload, _ := json.Marshal(req.Payload)

	containerCfg := &container.Config{
		Image:        req.ImageURI,
		Cmd:          []string{"python", "-m", req.Entrypoint},
		WorkingDir:   "/app",
		AttachStdout: true,
		AttachStderr: true,
		Env: []string{
			fmt.Sprintf("NIMBUS_PAYLOAD=%s", string(payload)),
			fmt.Sprintf("NIMBUS_TIMEOUT=%d", req.TimeoutSeconds),
		},
	}

	maxPIDs := int64(256)
	if req.MaxPIDsValue > 0 {
		maxPIDs = int64(req.MaxPIDsValue)
	}

	hostCfg := &container.HostConfig{
		CapDrop:        []string{"ALL"},
		ReadonlyRootfs: true,
		NetworkMode:    "none",
		Mounts: []mount.Mount{
			{Type: mount.TypeTmpfs, Target: "/tmp"},
		},
		AutoRemove: true,
		Resources: container.Resources{
			Memory:    int64(req.MemoryLimitMB) * 1024 * 1024,
			NanoCPUs:  int64(req.CPULimitMillicores) * 1e6,
			PidsLimit: &maxPIDs,
		},
	}

	resp, err := r.cli.ContainerCreate(ctx, client.ContainerCreateOptions{
		Config:     containerCfg,
		HostConfig: hostCfg,
		Name:       req.ContainerName,
	})
	if err != nil {
		return &ContainerResult{Err: fmt.Errorf("creating container: %w", err)}
	}
	containerID := resp.ID

	if _, err := r.cli.ContainerStart(ctx, containerID, client.ContainerStartOptions{}); err != nil {
		return &ContainerResult{Err: fmt.Errorf("starting container: %w", err)}
	}

	var stdout, stderr bytes.Buffer
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		_, _ = io.Copy(&stdout, &bytes.Buffer{})
	}()

	waitResult := r.cli.ContainerWait(ctx, containerID, client.ContainerWaitOptions{
		Condition: container.WaitConditionNotRunning,
	})
	select {
	case <-ctx.Done():
		r.killContainer(ctx, containerID)
		return &ContainerResult{Err: ctx.Err(), Duration: time.Since(start)}
	case err := <-waitResult.Error:
		r.killContainer(ctx, containerID)
		return &ContainerResult{Err: err, Duration: time.Since(start)}
	case result := <-waitResult.Result:
		wg.Wait()
		return &ContainerResult{
			Stdout:   stdout.String(),
			Stderr:   stderr.String(),
			ExitCode: int(result.StatusCode),
			Duration: time.Since(start),
		}
	}
}

func (r *Runner) pullImage(ctx context.Context, imageURI string) error {
	_, err := r.cli.ImagePull(ctx, imageURI, client.ImagePullOptions{})
	return err
}

func (r *Runner) killContainer(ctx context.Context, id string) {
	r.cli.ContainerKill(ctx, id, client.ContainerKillOptions{Signal: "SIGKILL"})
}

// RunRequest describes a function execution request.
type RunRequest struct {
	InvocationID       uuid.UUID
	ImageURI           string
	Entrypoint         string
	Payload            map[string]interface{}
	MemoryLimitMB      int
	CPULimitMillicores int
	TimeoutSeconds     int
	ContainerName      string
	MaxPIDsValue       int
}
