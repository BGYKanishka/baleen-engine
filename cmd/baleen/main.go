package main

import (
	"fmt"
	"os"
)

// Select the appropriate subcommand based on the first argument.
func main() {
	SelfCleanupIfExtensionRemoved()
	go AutoUpdateCLIPlugin()
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "status":
			runStatus()
			return
		case "daemon":
			runDaemon(os.Args[2:])
			return
		case "docker-cli-plugin-metadata":
			fmt.Println(`{"SchemaVersion":"0.1.0","Vendor":"Baleen","Version":"v1.0.0","ShortDescription":"Baleen P2P Image Sharing"}`)
			return
		}
	}

	// Default client mode.
	var args []string
	if len(os.Args) > 1 {
		args = os.Args[1:]
	}
	runClient(args)
}
