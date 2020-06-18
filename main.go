package main

import (
	"context"
	"flag"
	"fmt"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
)

var (
	shortUpdateImgFlag = flag.Bool("u", false, "Update the image before recreating the container")
	updateImgFlag      = flag.Bool("update", false, "Update the image before recreating the container")
)

func main() {
	flag.Parse()

	ctx := context.Background()
	containerID := flag.Arg(0)

	// Combine results from full flag and short flag
	shouldUpdateImageFlag := *shortUpdateImgFlag || *updateImgFlag

	cli, err := client.NewEnvClient()
	if err != nil {
		panic(err)
	}

	container, err := cli.ContainerInspect(ctx, containerID)
	if err != nil {
		panic(err)
	}

	isRunning := container.State.Running || container.State.Paused
	imageName := container.Config.Image

	// if container is set to autoremove stopping it would completely remove it
	if container.HostConfig.AutoRemove {
		panic(fmt.Errorf("container %s is set to autoremove", container.Name))
	}

	if isRunning {
		fmt.Println("Stopping container ...")

		err = cli.ContainerStop(ctx, containerID, nil)
		if err != nil {
			panic(err)
		}
	}

	if shouldUpdateImageFlag {
		fmt.Println("Updating container image ...")
		out, err := cli.ImagePull(ctx, imageName, types.ImagePullOptions{})
		if err != nil {
			panic(err)
		}

		defer out.Close()
	}

	fmt.Println("Starting container ...")

	err = cli.ContainerStart(ctx, containerID, types.ContainerStartOptions{})
	if err != nil {
		panic(err)
	}
}
