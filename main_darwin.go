package main

import (
	"fmt"
	"os"
	"time"
)

func main() {
	if err := serveMacFrames(os.Stdin, os.Stdout, time.Now); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
