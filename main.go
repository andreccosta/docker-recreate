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

	recreateCmd := flag.NewFlagSet("recreate", flag.ExitOnError)
	pullFlag := recreateCmd.Bool("pull", false, "Pull image before recreating the container")

	switch os.Args[1] {
	case "docker-cli-plugin-metadata":
		writer := json.NewEncoder(os.Stdout)
		writer.Encode(pluginMetadata)
	case "recreate":
		recreateCmd.Parse(os.Args[2:])
		tail := recreateCmd.Args()

		if len(tail) < 1 {
			fmt.Print("no container id or name specified\n")
			os.Exit(1)
		}
		
		fmt.Println("tail:", recreateCmd.Args())
		
		err := recreateContainer(tail[0], *pullFlag)
		if err != nil {
			panic(err)
		}
	default:
		recreateCmd.Parse(os.Args[1:])
		tail := recreateCmd.Args()

		if len(tail) < 1 {
			fmt.Print("no container id or name specified\n")
			os.Exit(1)
		}
		
		err := recreateContainer(tail[0], *pullFlag)
		if err != nil {
			panic(err)
		}
	}
}

func recreateContainer(containerID string, pullFlag bool) error {
	ctx := context.Background()

	cli, err := client.NewClientWithOpts()
	if err != nil {
		return err
	}

	originalContainer, err := cli.ContainerInspect(ctx, containerID)
	if err != nil {
		return err
	}

	if pullFlag {
		image := originalContainer.Config.Image
		platform := originalContainer.Platform

		fmt.Printf("Pulling image %s ...\n", image)
		out, err := cli.ImagePull(ctx, image, types.ImagePullOptions{
			Platform: platform,
		})
		if err != nil {
			return err
		}

		defer out.Close()
	}

	if originalContainer.State.Running || originalContainer.State.Paused {
		fmt.Printf("Stopping container %s ...\n", containerID)

		err = cli.ContainerStop(ctx, containerID, nil)
		if err != nil {
			return err
		}
	}

	if !originalContainer.HostConfig.AutoRemove {
		fmt.Printf("Removing container %s ...\n", containerID)

		err = cli.ContainerRemove(ctx, containerID, types.ContainerRemoveOptions{})
		if err != nil {
			return err
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
		return err
	}

	fmt.Printf("Starting container %s ...\n", createdContainer.ID[:10])
	err = cli.ContainerStart(ctx, createdContainer.ID, types.ContainerStartOptions{})
	if err != nil {
		return err
	}

	return nil
}
