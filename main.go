package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/client"
	v1 "github.com/opencontainers/image-spec/specs-go/v1"
)

var version = "dev"

var errHelp = errors.New("help requested")

type dockerClient interface {
	ContainerInspect(ctx context.Context, containerID string) (container.InspectResponse, error)
	ImageInspect(ctx context.Context, imageID string, inspectOpts ...client.ImageInspectOption) (image.InspectResponse, error)
	ImagePull(ctx context.Context, ref string, options image.PullOptions) (io.ReadCloser, error)
	ContainerStop(ctx context.Context, containerID string, options container.StopOptions) error
	ContainerRemove(ctx context.Context, containerID string, options container.RemoveOptions) error
	ContainerWait(ctx context.Context, containerID string, condition container.WaitCondition) (<-chan container.WaitResponse, <-chan error)
	ContainerCreate(ctx context.Context, config *container.Config, hostConfig *container.HostConfig, networkingConfig *network.NetworkingConfig, platform *v1.Platform, containerName string) (container.CreateResponse, error)
	ContainerStart(ctx context.Context, containerID string, options container.StartOptions) error
}

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr, newDockerClient))
}

func run(args []string, stdout io.Writer, stderr io.Writer, newClient func() (dockerClient, error)) int {
	if err := runCommand(args, stdout, newClient); err != nil {
		if errors.Is(err, errHelp) {
			return 0
		}

		fmt.Fprintln(stderr, err)
		return 1
	}

	return 0
}

func runCommand(args []string, stdout io.Writer, newClient func() (dockerClient, error)) error {
	if len(args) == 0 {
		return errors.New("no container id or name specified")
	}

	pluginMetadata := map[string]interface{}{
		"SchemaVersion":    "0.1.0",
		"Vendor":           "github.com/andreccosta",
		"Version":          strings.TrimPrefix(version, "v"),
		"ShortDescription": "Recreate containers",
		"Experimental":     true,
	}

	if args[0] == "docker-cli-plugin-metadata" {
		return json.NewEncoder(stdout).Encode(pluginMetadata)
	}

	targetArgs := args
	if args[0] == "recreate" {
		targetArgs = args[1:]
	}

	recreateCmd := flag.NewFlagSet("recreate", flag.ContinueOnError)
	recreateCmd.SetOutput(io.Discard)
	pullFlag := recreateCmd.Bool("pull", false, "Pull image before recreating the container")

	if err := recreateCmd.Parse(targetArgs); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			recreateCmd.SetOutput(stdout)
			recreateCmd.Usage()
			return errHelp
		}

		return fmt.Errorf("parse arguments: %w", err)
	}

	tail := recreateCmd.Args()
	if len(tail) < 1 {
		return errors.New("no container id or name specified")
	}

	cli, err := newClient()
	if err != nil {
		return fmt.Errorf("create docker client: %w", err)
	}

	if closer, ok := cli.(interface{ Close() error }); ok {
		defer closer.Close()
	}

	return recreateContainer(context.Background(), cli, stdout, tail[0], *pullFlag)
}

func newDockerClient() (dockerClient, error) {
	return client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
}

func recreateContainer(ctx context.Context, cli dockerClient, stdout io.Writer, containerID string, pullFlag bool) error {
	originalContainer, err := cli.ContainerInspect(ctx, containerID)
	if err != nil {
		return fmt.Errorf("inspect container %s: %w", containerID, err)
	}

	platform, err := resolvePlatform(ctx, cli, originalContainer)
	if err != nil {
		return err
	}

	if pullFlag {
		img := originalContainer.Config.Image

		fmt.Fprintf(stdout, "Pulling image %s ...\n", img)
		out, err := cli.ImagePull(ctx, img, image.PullOptions{Platform: formatPlatform(platform)})
		if err != nil {
			return fmt.Errorf("pull image %s: %w", img, err)
		}

		defer out.Close()

		if _, err := io.Copy(io.Discard, out); err != nil {
			return fmt.Errorf("read image pull response: %w", err)
		}
	}

	wasRunning := originalContainer.State != nil && (originalContainer.State.Running || originalContainer.State.Paused)
	if wasRunning {
		fmt.Fprintf(stdout, "Stopping container %s ...\n", containerID)

		if err := cli.ContainerStop(ctx, containerID, container.StopOptions{}); err != nil {
			return fmt.Errorf("stop container %s: %w", containerID, err)
		}
	}

	if originalContainer.HostConfig.AutoRemove && wasRunning {
		waitCh, errCh := cli.ContainerWait(ctx, containerID, container.WaitConditionRemoved)

		select {
		case waitResp := <-waitCh:
			if waitResp.Error != nil {
				return fmt.Errorf("wait for container %s removal: %s", containerID, waitResp.Error.Message)
			}
		case err := <-errCh:
			if err != nil {
				return fmt.Errorf("wait for container %s removal: %w", containerID, err)
			}
		}
	} else {
		fmt.Fprintf(stdout, "Removing container %s ...\n", containerID)

		if err := cli.ContainerRemove(ctx, containerID, container.RemoveOptions{}); err != nil {
			return fmt.Errorf("remove container %s: %w", containerID, err)
		}
	}

	name := normalizeContainerName(originalContainer.Name)
	networkingConfig := recreateNetworkingConfig(originalContainer.NetworkSettings)

	fmt.Fprintln(stdout, "Recreating container ...")
	createdContainer, err := cli.ContainerCreate(
		ctx,
		originalContainer.Config,
		originalContainer.HostConfig,
		networkingConfig,
		platform,
		name,
	)
	if err != nil {
		return fmt.Errorf("create replacement container for %s: %w", containerID, err)
	}

	fmt.Fprintf(stdout, "Starting container %s ...\n", createdContainer.ID[:10])
	if err := cli.ContainerStart(ctx, createdContainer.ID, container.StartOptions{}); err != nil {
		return fmt.Errorf("start container %s: %w", createdContainer.ID, err)
	}

	return nil
}

func resolvePlatform(ctx context.Context, cli dockerClient, inspected container.InspectResponse) (*v1.Platform, error) {
	if inspected.ImageManifestDescriptor != nil && inspected.ImageManifestDescriptor.Platform != nil {
		platform := *inspected.ImageManifestDescriptor.Platform
		return &platform, nil
	}

	imageDetails, err := cli.ImageInspect(ctx, inspected.Image)
	if err != nil {
		return nil, fmt.Errorf("inspect image %s: %w", inspected.Image, err)
	}

	platform := &v1.Platform{
		Architecture: imageDetails.Architecture,
		OS:           imageDetails.Os,
		OSVersion:    imageDetails.OsVersion,
		Variant:      imageDetails.Variant,
	}

	if platform.OS == "" && platform.Architecture == "" && platform.Variant == "" && platform.OSVersion == "" {
		return nil, nil
	}

	return platform, nil
}

func formatPlatform(platform *v1.Platform) string {
	if platform == nil || platform.OS == "" || platform.Architecture == "" {
		return ""
	}

	value := platform.OS + "/" + platform.Architecture
	if platform.Variant != "" {
		value += "/" + platform.Variant
	}

	return value
}

func normalizeContainerName(name string) string {
	return strings.TrimPrefix(name, "/")
}

func recreateNetworkingConfig(settings *container.NetworkSettings) *network.NetworkingConfig {
	if settings == nil || len(settings.Networks) == 0 {
		return nil
	}

	return &network.NetworkingConfig{EndpointsConfig: settings.Networks}
}
