package main

import (
	"os"

	"github.com/gloamers/openproject-bridge/internal/app/bootstrapapp"
)

func main() {
	os.Exit(bootstrapapp.Main(os.Args[1:]))
}
