package main

import (
	"os"

	"github.com/gloamers/openproject-bridge/internal/app/bridge"
)

func main() {
	os.Exit(bridge.Main(os.Args[1:]))
}
