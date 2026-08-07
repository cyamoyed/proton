package plugin

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestResolveNodeUsesManifestScriptsAndPackageManager(t *testing.T) {
	dir := t.TempDir()
	manifest := `{"scripts":{"dev":"vite","custom":"echo ok"}}`
	if err := os.WriteFile(filepath.Join(dir, "package.json"), []byte(manifest), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "pnpm-lock.yaml"), nil, 0644); err != nil {
		t.Fatal(err)
	}
	source := &Plugin{Name: "nodejs", Commands: []Command{{Name: "Build", Run: "npm run build"}}, Env: []EnvCheck{{Name: "npm", Check: "npm --version"}}}
	resolved := ResolveForDir(source, dir)
	joined := ""
	for _, command := range resolved.Commands {
		joined += command.Run + "\n"
	}
	if strings.Contains(joined, "build") || !strings.Contains(joined, "pnpm run dev") || !strings.Contains(joined, "pnpm run custom") {
		t.Fatalf("unexpected resolved commands:\n%s", joined)
	}
	if resolved.Env[0].Name != "pnpm" {
		t.Fatalf("environment = %+v, want pnpm", resolved.Env)
	}
}

func TestResolveCppMatchesBuildSystem(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "meson.build"), nil, 0644); err != nil {
		t.Fatal(err)
	}
	resolved := ResolveForDir(&Plugin{Name: "cpp", Commands: []Command{{Name: "Build", Run: "make"}}}, dir)
	if len(resolved.Commands) != 1 || !strings.Contains(resolved.Commands[0].Run, "meson") {
		t.Fatalf("commands = %+v, want Meson only", resolved.Commands)
	}
}
