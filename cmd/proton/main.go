package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/fupeijun/proton/internal/core"
	"github.com/fupeijun/proton/internal/installer"
)

var version = "dev"

func main() {
	if shouldInstall() {
		fmt.Println("Installing Proton...")
		target, err := installer.InstallWithProgress(func(message string) { fmt.Println("  " + message) })
		if err != nil {
			fmt.Fprintf(os.Stderr, "Installation failed: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Proton installed successfully: %s\n", target)
		fmt.Println("Open a new terminal, navigate to a project, and run: proton")
		if launchedAsInstaller() {
			fmt.Println("\nPress Enter to close this window...")
			_, _ = fmt.Scanln()
		}
		return
	}
	if len(os.Args) == 2 && os.Args[1] == "uninstall" {
		fmt.Println("Uninstalling Proton...")
		target, err := installer.UninstallWithProgress(func(message string) { fmt.Println("  " + message) })
		if err != nil {
			fmt.Fprintf(os.Stderr, "Uninstall failed: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Proton uninstalled: %s\n", target)
		return
	}
	depth := flag.Int("depth", 3, "maximum subdirectory scan depth (0 scans only the target directory)")
	showVersion := flag.Bool("version", false, "print version and exit")
	noRecursive := flag.Bool("no-recursive", false, "scan only the target directory")
	flag.Usage = func() {
		fmt.Fprintf(flag.CommandLine.Output(), "Proton detects local projects and launches relevant commands.\n\nUsage:\n  proton [options] [directory]\n  proton install\n  proton uninstall\n\nOptions:\n")
		flag.PrintDefaults()
	}
	flag.Parse()
	if *showVersion {
		fmt.Println("proton", version)
		return
	}
	if flag.NArg() > 1 {
		flag.Usage()
		os.Exit(2)
	}
	if *depth < 0 {
		fmt.Fprintln(os.Stderr, "Error: --depth must be zero or greater")
		os.Exit(2)
	}
	dir := "."
	if flag.NArg() == 1 {
		dir = flag.Arg(0)
	}
	if *noRecursive {
		*depth = 0
	}
	engine := core.NewEngine()
	if err := engine.RunWithOptions(core.Options{Dir: dir, Depth: *depth}); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func shouldInstall() bool {
	if len(os.Args) == 2 && os.Args[1] == "install" {
		return true
	}
	if len(os.Args) != 1 {
		return false
	}
	return launchedAsInstaller()
}

func launchedAsInstaller() bool {
	return len(os.Args) == 1 && strings.Contains(strings.ToLower(filepath.Base(os.Args[0])), "installer")
}
