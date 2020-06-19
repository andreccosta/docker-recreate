package main

import (
	"context"
	"flag"
	"fmt"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/client"
)

var (
	shortPullImgFlag = flag.Bool("p", false, "Pull image before recreating the container")
	pullImgFlag      = flag.Bool("pull", false, "Pull image before recreating the container")
)

func main() {
	flag.Parse()

	ctx := context.Background()
	containerID := flag.Arg(0)

	// combine results from full flag and short flag
	shouldPullImageFlag := *shortPullImgFlag || *pullImgFlag

	cli, err := client.NewEnvClient()
	if err != nil {
		panic(err)
	}

	cnt, err := cli.ContainerInspect(ctx, containerID)
	if err != nil {
		panic(err)
	}

	if shouldPullImageFlag {
		imageName := cnt.Config.Image

		fmt.Printf("Pulling image %s ...\n", imageName)
		out, err := cli.ImagePull(ctx, imageName, types.ImagePullOptions{})
		if err != nil {
			panic(err)
		}

		defer out.Close()
	}

	if cnt.State.Running || cnt.State.Paused {
		fmt.Printf("Stopping container %s ...\n", containerID)

		err = cli.ContainerStop(ctx, containerID, nil)
		if err != nil {
			panic(err)
		}
	}

	if !cnt.HostConfig.AutoRemove {
		fmt.Printf("Removing container %s ...\n", containerID)

		err = cli.ContainerRemove(ctx, containerID, types.ContainerRemoveOptions{})
		if err != nil {
			panic(err)
		}
	}

	fmt.Println("Recreating container ...")
	createdContainer, err := cli.ContainerCreate(
		ctx,
		cnt.Config,
		cnt.HostConfig,
		&network.NetworkingConfig{
			EndpointsConfig: cnt.NetworkSettings.Networks,
		},
		cnt.Name)
	if err != nil {
		panic(err)
	}

	fmt.Printf("Starting container %s ...\n", createdContainer.ID[:10])
	err = cli.ContainerStart(ctx, createdContainer.ID, types.ContainerStartOptions{})
	if err != nil {
		panic(err)
	}
}
