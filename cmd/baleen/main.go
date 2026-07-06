package main

import (
	"os"
)

// Select the appropriate subcommand based on the first argument.
func main() {
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "status":
			runStatus()
			return
		case "daemon":
			runDaemon(os.Args[2:])
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
