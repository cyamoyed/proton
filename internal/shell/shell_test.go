package shell

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestNeedsPowerShell(t *testing.T) {
	// Commands that should use cmd.exe (no PowerShell needed)
	for _, cmd := range []string{
		"npm run start",
		"npm test",
		"go build ./...",
		"python main.py",
		"cargo build",
		"make all",
		"echo hello world",
	} {
		if needsPowerShell(cmd) {
			t.Errorf("needsPowerShell(%q) = true, want false", cmd)
		}
	}
	// Commands that should use PowerShell
	for _, cmd := range []string{
		"Get-Content file.txt",
		"echo $HOME",
		"$_ = 'test'",
		"Invoke-WebRequest url",
		"where-Object { $_.Name -eq 'foo' }",
		"@('a','b') | ForEach-Object { $_ }",
	} {
		if !needsPowerShell(cmd) {
			t.Errorf("needsPowerShell(%q) = false, want true", cmd)
		}
	}
}

func TestShellCommandSelectsWindowsShell(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("windows-only shell selection")
	}
	name, args := ShellCommand("npm run start")
	if name != "cmd.exe" || len(args) != 2 || args[0] != "/c" || args[1] != "npm run start" {
		t.Errorf("ShellCommand(simple) = %s %v, want cmd.exe /c <command>", name, args)
	}
	name, args = ShellCommand("echo $HOME")
	if name != "powershell.exe" {
		t.Errorf("ShellCommand(powershell syntax) = %s, want powershell.exe", name)
	}
	want := []string{"-NoProfile", "-ExecutionPolicy", "Bypass", "-Command", "echo $HOME"}
	if len(args) != len(want) {
		t.Fatalf("powershell args = %v, want %v", args, want)
	}
	for i := range want {
		if args[i] != want[i] {
			t.Errorf("powershell args[%d] = %q, want %q", i, args[i], want[i])
		}
	}
}

func TestCommandEnvLoadsDotenvWithoutOverriding(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ".env"), []byte("PROTON_SHELL_TEST_NEW=from_dotenv\nPROTON_SHELL_TEST_EXISTING=from_dotenv\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PROTON_SHELL_TEST_EXISTING", "from_process")

	env, err := CommandEnv(dir)
	if err != nil {
		t.Fatalf("CommandEnv() error = %v", err)
	}
	values := make(map[string]string)
	for _, kv := range env {
		for i := 0; i < len(kv); i++ {
			if kv[i] == '=' {
				values[kv[:i]] = kv[i+1:]
				break
			}
		}
	}
	if values["PROTON_SHELL_TEST_NEW"] != "from_dotenv" {
		t.Errorf("new .env variable missing or wrong: %q", values["PROTON_SHELL_TEST_NEW"])
	}
	if values["PROTON_SHELL_TEST_EXISTING"] != "from_process" {
		t.Errorf("existing process variable was overridden: %q", values["PROTON_SHELL_TEST_EXISTING"])
	}
}

func TestCommandEnvWithoutDotenv(t *testing.T) {
	env, err := CommandEnv(t.TempDir())
	if err != nil {
		t.Fatalf("CommandEnv() error = %v", err)
	}
	if env != nil {
		t.Errorf("CommandEnv() = %v, want nil without .env", env)
	}
}
