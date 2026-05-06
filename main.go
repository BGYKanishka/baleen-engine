package main

import (
	"context"
	"fmt"

	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/client"
)

func main() {
	fmt.Println("Starting Baleen Engine (Go Edition)...")

	// Connect to the local docker daemon
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())

	if err != nil {
		panic(err)
	}
	defer func(cli *client.Client) {
		err := cli.Close()
		if err != nil {

		}
	}(cli)

	// list of images
	images, err := cli.ImageList(context.Background(), image.ListOptions{})
	if err != nil {
		panic(err)
	}

	fmt.Printf("Connected to Docker! Found %d images on your Mac.\n", len(images))

	for _, img := range images {
		if len(img.RepoTags) > 0 {
			sizeInMB := img.Size / 1024 / 1024
			fmt.Printf("- %s (Size: %d MB)\n", img.RepoTags[0], sizeInMB)
		}
	}
}
