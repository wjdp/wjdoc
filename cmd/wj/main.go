package main

import (
	"fmt"
	"os"
)

func main() {
	if err := rootCommand().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "wj:", err)
		os.Exit(1)
	}
}
