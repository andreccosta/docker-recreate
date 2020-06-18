package main

import (
	"flag"
	"fmt"
)

var (
	shortUpdateImgFlag = flag.Bool("u", false, "Update the image before recreating the container")
	updateImgFlag      = flag.Bool("update", false, "Update the image before recreating the container")
)

func main() {
	flag.Parse()

	// Combine results from full flag and short flag
	shouldUpdateImageFlag := *shortUpdateImgFlag || *updateImgFlag

	fmt.Println("Combined flag value", shouldUpdateImageFlag)
}
