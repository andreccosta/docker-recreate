package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/client"
	v1 "github.com/opencontainers/image-spec/specs-go/v1"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Print("no container id or name specified\n")
		os.Exit(1)
	}

	var pluginMetadata = map[string]interface{}{
		"SchemaVersion":    "0.1.0",
		"Vendor":           "github.com/andreccosta",
		"Version":          "0.1.2",
		"ShortDescription": "Recreate containers",
		"Experimental":     true,
	}

	recreateCmd := flag.NewFlagSet("", flag.ExitOnError)
	pullFlag := recreateCmd.Bool("pull", false, "Pull image before recreating the container")

	switch os.Args[1] {
	case "docker-cli-plugin-metadata":
		writer := json.NewEncoder(os.Stdout)
		writer.Encode(pluginMetadata)
	case "recreate":
		if len(os.Args) < 3 {
			fmt.Print("no container id or name specified\n")
			os.Exit(1)
		}

		recreateCmd.Parse(os.Args[3:])
		recreateContainer(os.Args[2], *pullFlag)
	default:
		recreateCmd.Parse(os.Args[2:])
		recreateContainer(os.Args[1], *pullFlag)
	}
}

func recreateContainer(containerID string, pullFlag bool) {
	ctx := context.Background()

	cli, err := client.NewEnvClient()
	if err != nil {
		panic(err)
	}

	originalContainer, err := cli.ContainerInspect(ctx, containerID)
	if err != nil {
		panic(err)
	}

	if pullFlag {
		imageName := originalContainer.Config.Image

		fmt.Printf("Pulling image %s ...\n", imageName)
		out, err := cli.ImagePull(ctx, imageName, types.ImagePullOptions{})
		if err != nil {
			panic(err)
		}

		defer out.Close()
	}

	if originalContainer.State.Running || originalContainer.State.Paused {
		fmt.Printf("Stopping container %s ...\n", containerID)

		err = cli.ContainerStop(ctx, containerID, nil)
		if err != nil {
			panic(err)
		}
	}

	if !originalContainer.HostConfig.AutoRemove {
		fmt.Printf("Removing container %s ...\n", containerID)

		err = cli.ContainerRemove(ctx, containerID, types.ContainerRemoveOptions{})
		if err != nil {
			panic(err)
		}
	}

	fmt.Println("Recreating container ...")
	createdContainer, err := cli.ContainerCreate(
		ctx,
		originalContainer.Config,
		originalContainer.HostConfig,
		&network.NetworkingConfig{
			EndpointsConfig: originalContainer.NetworkSettings.Networks,
		},
		&v1.Platform{
			OS: originalContainer.Platform,
		},
		originalContainer.Name)
	if err != nil {
		panic(err)
	}

	fmt.Printf("Starting container %s ...\n", createdContainer.ID[:10])
	err = cli.ContainerStart(ctx, createdContainer.ID, types.ContainerStartOptions{})
	if err != nil {
		panic(err)
	}
}
