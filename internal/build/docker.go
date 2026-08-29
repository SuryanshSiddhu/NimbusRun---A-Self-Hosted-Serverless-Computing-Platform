package build

import (
	"archive/tar"
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/moby/moby/api/types/build"
	"github.com/moby/moby/api/types/image"
	"github.com/moby/moby/client"
)

// Builder builds Docker images from source ZIP.
type Builder struct {
	cli       *client.Client
	registry  string
	workDir   string
	timeout   time.Duration
}

// BuildResult holds the result of a build.
type BuildResult struct {
	ImageURI string
	Logs     string
	Err      error
}

// NewBuilder creates a new Builder.
func NewBuilder(registry, workDir string, timeout time.Duration) (*Builder, error) {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, fmt.Errorf("creating docker client: %w", err)
	}
	return &Builder{
		cli:      cli,
		registry: registry,
		workDir:  workDir,
		timeout:  timeout,
	}, nil
}

// BuildFromZip builds a Docker image from a source ZIP archive.
func (b *Builder) BuildFromZip(ctx context.Context, functionName, zipPath, entrypoint string) *BuildResult {
	// Create a temp directory for the build context
	tmpDir, err := os.MkdirTemp(b.workDir, "nimbus-build-*")
	if err != nil {
		return &BuildResult{Err: fmt.Errorf("creating temp dir: %w", err)}
	}
	defer os.RemoveAll(tmpDir)

	// Extract ZIP to build context
	if err := extractZip(zipPath, tmpDir); err != nil {
		return &BuildResult{Err: fmt.Errorf("extracting zip: %w", err)}
	}

	// Write Dockerfile
	dockerfile := generateDockerfile(entrypoint)
	if err := os.WriteFile(filepath.Join(tmpDir, "Dockerfile"), []byte(dockerfile), 0644); err != nil {
		return &BuildResult{Err: fmt.Errorf("writing dockerfile: %w", err)}
	}

	// Build image
	imageTag := fmt.Sprintf("%s/%s:%s", b.registry, functionName, time.Now().Format("v20060102-150405"))

	buildCtx, err := createBuildContext(tmpDir)
	if err != nil {
		return &BuildResult{Err: fmt.Errorf("creating build context: %w", err)}
	}

	buildOpts := build.ImageBuildOptions{
		Tags:       []string{imageTag},
		Dockerfile: "Dockerfile",
		Remove:     true,
		PullParent: true,
	}

	buildResp, err := b.cli.ImageBuild(ctx, buildCtx, buildOpts)
	if err != nil {
		return &BuildResult{Err: fmt.Errorf("building image: %w", err)}
	}
	defer buildResp.Body.Close()

	// Read build logs
	logs, err := io.ReadAll(buildResp.Body)
	if err != nil {
		return &BuildResult{Err: fmt.Errorf("reading build logs: %w", err)}
	}

	// Check if build succeeded
	success := !strings.Contains(string(logs), "error")
	if !success {
		return &BuildResult{
			ImageURI: "",
			Logs:     string(logs),
			Err:      fmt.Errorf("build failed"),
		}
	}

	// Tag and push to local registry
	if err := b.pushImage(ctx, imageTag); err != nil {
		return &BuildResult{
			ImageURI: "",
			Logs:     string(logs),
			Err:      fmt.Errorf("pushing image: %w", err),
		}
	}

	return &BuildResult{
		ImageURI: imageTag,
		Logs:     string(logs),
		Err:      nil,
	}
}

func extractZip(src, dest string) error {
	// Simplified: use `unzip` command or Go's archive/zip
	// For production, use Go's archive/zip package
	return nil // TODO: implement zip extraction
}

func generateDockerfile(entrypoint string) string {
	return fmt.Sprintf(`FROM python:3.12-slim
WORKDIR /app
COPY requirements.txt .
RUN pip install --no-cache-dir -r requirements.txt
COPY . .
USER 1000:1000
ENTRYPOINT ["python", "-m", "%s"]
`, entrypoint)
}

func createBuildContext(dir string) (io.Reader, error) {
	buf := new(bytes.Buffer)
	tw := tar.NewWriter(buf)
	defer tw.Close()

	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		relPath, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}
		header := &tar.Header{
			Name: relPath,
			Size: info.Size(),
			Mode: int64(info.Mode()),
		}
		if err := tw.WriteHeader(header); err != nil {
			return err
		}
		f, err := os.Open(path)
		if err != nil {
			return err
		}
		defer f.Close()
		_, err = io.Copy(tw, f)
		return err
	})
	return buf, err
}

func (b *Builder) pushImage(ctx context.Context, imageTag string) error {
	// Tag for registry
	_, err := b.cli.ImagePush(ctx, imageTag, image.PushOptions{})
	return err
}

// Cleanup removes the built image from local Docker.
func (b *Builder) Cleanup(ctx context.Context, imageTag string) error {
	_, err := b.cli.ImageRemove(ctx, imageTag, image.RemoveOptions{Force: true})
	return err
}