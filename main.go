package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/client"
)

var (
	pullFlag = flag.Bool("pull", false, "Pull image before recreating the container")
)

func main() {
	flag.Parse()

	if flag.NArg() < 1 {
		fmt.Print("no container id or name specified\n")
		flag.Usage()
		os.Exit(2)
	}

	ctx := context.Background()
	containerID := flag.Arg(0)

	cli, err := client.NewEnvClient()
	if err != nil {
		panic(err)
	}

	originalContainer, err := cli.ContainerInspect(ctx, containerID)
	if err != nil {
		panic(err)
	}

	if *pullFlag {
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
