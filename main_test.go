package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/client"
	v1 "github.com/opencontainers/image-spec/specs-go/v1"
)

func TestRunMetadataUsesInjectedVersion(t *testing.T) {
	originalVersion := version
	version = "v1.2.3"
	t.Cleanup(func() {
		version = originalVersion
	})

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	exitCode := run([]string{"docker-cli-plugin-metadata"}, &stdout, &stderr, func() (dockerClient, error) {
		t.Fatal("docker client should not be created for metadata")
		return nil, nil
	})

	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d", exitCode)
	}

	if stderr.Len() != 0 {
		t.Fatalf("expected empty stderr, got %q", stderr.String())
	}

	var metadata map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &metadata); err != nil {
		t.Fatalf("unmarshal metadata: %v", err)
	}

	if metadata["Version"] != "1.2.3" {
		t.Fatalf("expected trimmed version, got %#v", metadata["Version"])
	}
	if metadata["ShortDescription"] != "Recreate containers" {
		t.Fatalf("unexpected metadata: %#v", metadata)
	}
}

func TestRunRequiresContainerName(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	exitCode := run([]string{"recreate"}, &stdout, &stderr, func() (dockerClient, error) {
		t.Fatal("docker client should not be created when args are invalid")
		return nil, nil
	})

	if exitCode != 1 {
		t.Fatalf("expected exit code 1, got %d", exitCode)
	}

	if stdout.Len() != 0 {
		t.Fatalf("expected empty stdout, got %q", stdout.String())
	}

	if got := strings.TrimSpace(stderr.String()); got != "no container id or name specified" {
		t.Fatalf("unexpected stderr: %q", got)
	}
}

func TestRunHelpPrintsUsageAndSucceeds(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	exitCode := run([]string{"--help"}, &stdout, &stderr, func() (dockerClient, error) {
		t.Fatal("docker client should not be created for help")
		return nil, nil
	})

	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d", exitCode)
	}

	if stderr.Len() != 0 {
		t.Fatalf("expected empty stderr, got %q", stderr.String())
	}

	if !strings.Contains(stdout.String(), "Usage of recreate:") {
		t.Fatalf("expected usage output, got %q", stdout.String())
	}
}

func TestRunDirectInvocationRecreatesContainer(t *testing.T) {
	client := &fakeDockerClient{
		inspectResponse: container.InspectResponse{
			ContainerJSONBase: &container.ContainerJSONBase{
				Image:      "sha256:test",
				Name:       "/demo",
				State:      &container.State{},
				HostConfig: &container.HostConfig{},
			},
			Config: &container.Config{Image: "alpine:latest"},
		},
		imageInspectResponse: image.InspectResponse{Os: "linux", Architecture: "arm64"},
		createResponse:       container.CreateResponse{ID: "1234567890abcdef"},
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	exitCode := run([]string{"demo"}, &stdout, &stderr, func() (dockerClient, error) {
		return client, nil
	})

	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d, stderr=%q", exitCode, stderr.String())
	}

	if stderr.Len() != 0 {
		t.Fatalf("expected empty stderr, got %q", stderr.String())
	}

	if client.inspectedContainerID != "demo" {
		t.Fatalf("expected inspect to use container arg, got %q", client.inspectedContainerID)
	}
	if client.removedContainerID != "demo" {
		t.Fatalf("expected container removal, got %q", client.removedContainerID)
	}
	if client.createdContainerName != "demo" {
		t.Fatalf("expected normalized name, got %q", client.createdContainerName)
	}
	if client.startedContainerID != "1234567890abcdef" {
		t.Fatalf("expected container start, got %q", client.startedContainerID)
	}
	if client.createPlatform == nil || client.createPlatform.OS != "linux" || client.createPlatform.Architecture != "arm64" {
		t.Fatalf("unexpected create platform: %#v", client.createPlatform)
	}
	if strings.Contains(stdout.String(), "tail:") {
		t.Fatalf("unexpected debug output: %q", stdout.String())
	}
}

func TestRecreateContainerPullsImageAndUsesDescriptorPlatform(t *testing.T) {
	pullBody := &trackingReadCloser{Reader: strings.NewReader("status")}
	client := &fakeDockerClient{
		inspectResponse: container.InspectResponse{
			ContainerJSONBase: &container.ContainerJSONBase{
				Image:      "sha256:test",
				Name:       "/demo",
				State:      &container.State{},
				HostConfig: &container.HostConfig{},
			},
			Config: &container.Config{Image: "repo/demo:latest"},
			ImageManifestDescriptor: &v1.Descriptor{
				Platform: &v1.Platform{OS: "linux", Architecture: "amd64", Variant: "v3"},
			},
			NetworkSettings: &container.NetworkSettings{
				Networks: map[string]*network.EndpointSettings{"bridge": {}},
			},
		},
		pullReadCloser: pullBody,
		createResponse: container.CreateResponse{ID: "abcdef1234567890"},
	}

	var stdout bytes.Buffer
	if err := recreateContainer(context.Background(), client, &stdout, "demo", true); err != nil {
		t.Fatalf("recreate container: %v", err)
	}

	if client.imageInspectCalls != 0 {
		t.Fatalf("expected image inspect to be skipped when descriptor platform exists, got %d", client.imageInspectCalls)
	}
	if client.pulledImageRef != "repo/demo:latest" {
		t.Fatalf("unexpected pulled image ref: %q", client.pulledImageRef)
	}
	if client.pullPlatform != "linux/amd64/v3" {
		t.Fatalf("unexpected pull platform: %q", client.pullPlatform)
	}
	if !pullBody.closed {
		t.Fatal("expected image pull body to be closed")
	}
	if pullBody.bytesRead == 0 {
		t.Fatal("expected image pull body to be consumed")
	}
	if client.createdNetworkingConfig == nil || client.createdNetworkingConfig.EndpointsConfig["bridge"] == nil {
		t.Fatalf("unexpected networking config: %#v", client.createdNetworkingConfig)
	}
	if client.createPlatform == nil || client.createPlatform.Variant != "v3" {
		t.Fatalf("unexpected create platform: %#v", client.createPlatform)
	}
}

func TestRecreateContainerWaitsForAutoRemoveContainer(t *testing.T) {
	waitCh := make(chan container.WaitResponse, 1)
	errCh := make(chan error, 1)
	waitCh <- container.WaitResponse{StatusCode: 0}

	client := &fakeDockerClient{
		inspectResponse: container.InspectResponse{
			ContainerJSONBase: &container.ContainerJSONBase{
				Image:      "sha256:test",
				Name:       "/demo",
				State:      &container.State{Running: true},
				HostConfig: &container.HostConfig{AutoRemove: true},
			},
			Config: &container.Config{Image: "repo/demo:latest"},
		},
		imageInspectResponse: image.InspectResponse{Os: "linux", Architecture: "amd64"},
		waitCh:               waitCh,
		errCh:                errCh,
		createResponse:       container.CreateResponse{ID: "abcdef1234567890"},
	}

	if err := recreateContainer(context.Background(), client, io.Discard, "demo", false); err != nil {
		t.Fatalf("recreate container: %v", err)
	}

	if client.stoppedContainerID != "demo" {
		t.Fatalf("expected stop call, got %q", client.stoppedContainerID)
	}
	if client.waitedContainerID != "demo" {
		t.Fatalf("expected wait for removal, got %q", client.waitedContainerID)
	}
	if client.removedContainerID != "" {
		t.Fatalf("did not expect explicit remove, got %q", client.removedContainerID)
	}
}

func TestRecreateContainerWaitErrorIncludesContext(t *testing.T) {
	waitCh := make(chan container.WaitResponse, 1)
	errCh := make(chan error)
	waitCh <- container.WaitResponse{Error: &container.WaitExitError{Message: "remove failed"}}

	client := &fakeDockerClient{
		inspectResponse: container.InspectResponse{
			ContainerJSONBase: &container.ContainerJSONBase{
				Image:      "sha256:test",
				Name:       "/demo",
				State:      &container.State{Running: true},
				HostConfig: &container.HostConfig{AutoRemove: true},
			},
			Config: &container.Config{Image: "repo/demo:latest"},
		},
		imageInspectResponse: image.InspectResponse{Os: "linux", Architecture: "amd64"},
		waitCh:               waitCh,
		errCh:                errCh,
	}

	err := recreateContainer(context.Background(), client, io.Discard, "demo", false)
	if err == nil {
		t.Fatal("expected wait error")
	}

	if got := err.Error(); got != "wait for container demo removal: remove failed" {
		t.Fatalf("unexpected error: %q", got)
	}
}

func TestResolvePlatformReturnsImageInspectError(t *testing.T) {
	client := &fakeDockerClient{imageInspectErr: errors.New("boom")}
	inspected := container.InspectResponse{
		ContainerJSONBase: &container.ContainerJSONBase{Image: "sha256:test"},
	}

	_, err := resolvePlatform(context.Background(), client, inspected)
	if err == nil || !strings.Contains(err.Error(), "inspect image sha256:test") {
		t.Fatalf("expected wrapped inspect error, got %v", err)
	}
}

type fakeDockerClient struct {
	inspectResponse      container.InspectResponse
	inspectErr           error
	imageInspectResponse image.InspectResponse
	imageInspectErr      error
	pullReadCloser       io.ReadCloser
	pullErr              error
	createResponse       container.CreateResponse
	createErr            error
	waitCh               <-chan container.WaitResponse
	errCh                <-chan error

	inspectedContainerID    string
	imageInspectCalls       int
	pulledImageRef          string
	pullPlatform            string
	stoppedContainerID      string
	removedContainerID      string
	waitedContainerID       string
	createdContainerName    string
	createdNetworkingConfig *network.NetworkingConfig
	createPlatform          *v1.Platform
	startedContainerID      string
}

func (f *fakeDockerClient) ContainerInspect(_ context.Context, containerID string) (container.InspectResponse, error) {
	f.inspectedContainerID = containerID
	return f.inspectResponse, f.inspectErr
}

func (f *fakeDockerClient) ImageInspect(context.Context, string, ...client.ImageInspectOption) (image.InspectResponse, error) {
	f.imageInspectCalls++
	return f.imageInspectResponse, f.imageInspectErr
}

func (f *fakeDockerClient) ImagePull(_ context.Context, ref string, options image.PullOptions) (io.ReadCloser, error) {
	f.pulledImageRef = ref
	f.pullPlatform = options.Platform
	if f.pullReadCloser == nil {
		f.pullReadCloser = io.NopCloser(strings.NewReader(""))
	}
	return f.pullReadCloser, f.pullErr
}

func (f *fakeDockerClient) ContainerStop(_ context.Context, containerID string, _ container.StopOptions) error {
	f.stoppedContainerID = containerID
	return nil
}

func (f *fakeDockerClient) ContainerRemove(_ context.Context, containerID string, _ container.RemoveOptions) error {
	f.removedContainerID = containerID
	return nil
}

func (f *fakeDockerClient) ContainerWait(_ context.Context, containerID string, _ container.WaitCondition) (<-chan container.WaitResponse, <-chan error) {
	f.waitedContainerID = containerID
	if f.waitCh == nil {
		waitCh := make(chan container.WaitResponse, 1)
		waitCh <- container.WaitResponse{StatusCode: 0}
		f.waitCh = waitCh
	}
	if f.errCh == nil {
		errCh := make(chan error, 1)
		f.errCh = errCh
	}
	return f.waitCh, f.errCh
}

func (f *fakeDockerClient) ContainerCreate(_ context.Context, _ *container.Config, _ *container.HostConfig, networkingConfig *network.NetworkingConfig, platform *v1.Platform, containerName string) (container.CreateResponse, error) {
	f.createdContainerName = containerName
	f.createdNetworkingConfig = networkingConfig
	if platform != nil {
		clone := *platform
		f.createPlatform = &clone
	}
	return f.createResponse, f.createErr
}

func (f *fakeDockerClient) ContainerStart(_ context.Context, containerID string, _ container.StartOptions) error {
	f.startedContainerID = containerID
	return nil
}

type trackingReadCloser struct {
	io.Reader
	bytesRead int
	closed    bool
}

func (r *trackingReadCloser) Read(p []byte) (int, error) {
	n, err := r.Reader.Read(p)
	r.bytesRead += n
	return n, err
}

func (r *trackingReadCloser) Close() error {
	r.closed = true
	return nil
}
