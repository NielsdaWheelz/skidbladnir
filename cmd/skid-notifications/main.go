package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"runtime"
	"syscall"

	"github.com/NielsdaWheelz/skidbladnir/internal/macnotifications"
)

var releaseVersion = "dev"
var releaseSHA = "unknown"

// Package initialization runs on the original startup OS thread. Locking only
// after main begins cannot establish AppKit's original-thread requirement.
func init() { runtime.LockOSThread() }

func main() {
	// Version inspection never creates an application, a socket, or a native
	// consent request.
	if len(os.Args) == 2 && os.Args[1] == "version" {
		fmt.Printf("%s %s\n", releaseVersion, releaseSHA)
		return
	}
	if len(os.Args) >= 2 && (os.Args[1] == "register" || os.Args[1] == "status" || os.Args[1] == "stop") {
		if len(os.Args) != 3 {
			fmt.Fprintf(os.Stderr, "usage: skid-notifications %s ABSOLUTE_APP_BUNDLE\n", os.Args[1])
			os.Exit(1)
		}
		var err error
		switch os.Args[1] {
		case "register":
			err = macnotifications.NativeRegister(os.Args[2])
		case "status":
			var pid int
			pid, err = macnotifications.NativeStatus(os.Args[2])
			if err == nil {
				fmt.Println(pid)
			}
		case "stop":
			err = macnotifications.NativeStopApplication(os.Args[2])
		}
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		return
	}
	configPath, skidPath, err := macnotifications.DefaultPaths()
	if err != nil {
		fmt.Fprintln(os.Stderr, "notifications unavailable")
		os.Exit(1)
	}
	flags := flag.NewFlagSet("skid-notifications", flag.ContinueOnError)
	config := flags.String("config", configPath, "private skid client configuration")
	skid := flags.String("skid", skidPath, "exact skid executable")
	if flags.Parse(os.Args[1:]) != nil || flags.NArg() != 0 {
		os.Exit(64)
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if macnotifications.Run(ctx, *config, *skid) != nil {
		fmt.Fprintln(os.Stderr, "notifications unavailable")
		os.Exit(1)
	}
}
