package main

import (
	"errors"
	"fmt"
	"os"

	"github.com/NielsdaWheelz/skidbladnir/internal/contract"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) == 0 {
		return errors.New("usage: skidbladnir-contract {generate|verify|accept {core|upgrade}}")
	}

	root, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get repository directory: %w", err)
	}

	switch args[0] {
	case "generate":
		if len(args) != 1 {
			return errors.New("usage: skidbladnir-contract generate")
		}
		return contract.Generate(root)
	case "verify":
		if len(args) != 1 {
			return errors.New("usage: skidbladnir-contract verify")
		}
		return contract.Verify(root)
	case "accept":
		if len(args) != 2 || (args[1] != "core" && args[1] != "upgrade") {
			return errors.New("usage: skidbladnir-contract accept {core|upgrade}")
		}
		return contract.Accept(root, args[1], os.Stdout)
	default:
		return fmt.Errorf("unknown contract command: %s", args[0])
	}
}
