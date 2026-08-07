package main

import (
	"os"
	"testing"
)

func TestShouldInstall(t *testing.T) {
	original := os.Args
	defer func() { os.Args = original }()

	os.Args = []string{"proton-installer-windows-amd64.exe"}
	if !shouldInstall() {
		t.Fatal("installer-named executable should install when run without arguments")
	}
	os.Args = []string{"proton", "install"}
	if !shouldInstall() {
		t.Fatal("install subcommand should trigger installation")
	}
	os.Args = []string{"proton"}
	if shouldInstall() {
		t.Fatal("installed proton command should launch normally")
	}
}
