package dotenv

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseBasic(t *testing.T) {
	data := []byte("KEY=value\nOTHER=hello world")
	env, err := Parse(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if env["KEY"] != "value" {
		t.Errorf("KEY = %q, want %q", env["KEY"], "value")
	}
	if env["OTHER"] != "hello world" {
		t.Errorf("OTHER = %q, want %q", env["OTHER"], "hello world")
	}
}

func TestParseComments(t *testing.T) {
	data := []byte("# this is a comment\nKEY=value\n# another comment\nOTHER=test")
	env, err := Parse(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(env) != 2 {
		t.Errorf("expected 2 entries, got %d", len(env))
	}
	if env["KEY"] != "value" {
		t.Errorf("KEY = %q, want %q", env["KEY"], "value")
	}
}

func TestParseInlineComment(t *testing.T) {
	data := []byte("KEY=value # this is a comment")
	env, err := Parse(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if env["KEY"] != "value" {
		t.Errorf("KEY = %q, want %q", env["KEY"], "value")
	}
}

func TestParseQuotedValues(t *testing.T) {
	tests := []struct {
		input string
		key   string
		want  string
	}{
		{`KEY="hello world"`, "KEY", "hello world"},
		{`KEY='hello world'`, "KEY", "hello world"},
		{`KEY="value with #hash"`, "KEY", "value with #hash"},
		{`KEY="line\nbreak"`, "KEY", "line\nbreak"},
		{`KEY="tab\there"`, "KEY", "tab\there"},
		{`KEY="back\\slash"`, "KEY", "back\\slash"},
		{`KEY="escaped\"quote"`, "KEY", `escaped"quote`},
	}
	for _, tt := range tests {
		t.Run(tt.key, func(t *testing.T) {
			env, err := Parse([]byte(tt.input))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if env[tt.key] != tt.want {
				t.Errorf("%s = %q, want %q", tt.key, env[tt.key], tt.want)
			}
		})
	}
}

func TestParseExportPrefix(t *testing.T) {
	data := []byte("export KEY=value\nexport OTHER=test")
	env, err := Parse(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if env["KEY"] != "value" {
		t.Errorf("KEY = %q, want %q", env["KEY"], "value")
	}
	if env["OTHER"] != "test" {
		t.Errorf("OTHER = %q, want %q", env["OTHER"], "test")
	}
}

func TestParseEmptyValue(t *testing.T) {
	data := []byte("KEY=\nOTHER=")
	env, err := Parse(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if env["KEY"] != "" {
		t.Errorf("KEY = %q, want empty", env["KEY"])
	}
	if env["OTHER"] != "" {
		t.Errorf("OTHER = %q, want empty", env["OTHER"])
	}
}

func TestParseEmptyLines(t *testing.T) {
	data := []byte("\n\nKEY=value\n\nOTHER=test\n\n")
	env, err := Parse(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(env) != 2 {
		t.Errorf("expected 2 entries, got %d", len(env))
	}
}

func TestParseVariableExpansion(t *testing.T) {
	t.Setenv("EXISTING", "from_env")
	data := []byte("BASE=hello\nCOMBINED=${BASE}_world\nREF=$EXISTING\nBOTH=${BASE}_${EXISTING}")
	env, err := Parse(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if env["COMBINED"] != "hello_world" {
		t.Errorf("COMBINED = %q, want %q", env["COMBINED"], "hello_world")
	}
	if env["REF"] != "from_env" {
		t.Errorf("REF = %q, want %q", env["REF"], "from_env")
	}
	if env["BOTH"] != "hello_from_env" {
		t.Errorf("BOTH = %q, want %q", env["BOTH"], "hello_from_env")
	}
}

func TestParseUndefinedVariable(t *testing.T) {
	data := []byte("KEY=${UNDEFINED_VAR}")
	env, err := Parse(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if env["KEY"] != "${UNDEFINED_VAR}" {
		t.Errorf("KEY = %q, want %q", env["KEY"], "${UNDEFINED_VAR}")
	}
}

func TestParseInvalidKey(t *testing.T) {
	tests := []string{
		"123=value",
		"-key=value",
		"key.with.dots=value",
	}
	for _, input := range tests {
		_, err := Parse([]byte(input))
		if err == nil {
			t.Errorf("expected error for %q, got nil", input)
		}
	}
}

func TestLoadFromDirExists(t *testing.T) {
	dir := t.TempDir()
	envFile := filepath.Join(dir, ".env")
	if err := os.WriteFile(envFile, []byte("FOO=bar\nBAZ=qux"), 0644); err != nil {
		t.Fatalf("failed to write .env: %v", err)
	}
	env, err := LoadFromDir(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if env["FOO"] != "bar" {
		t.Errorf("FOO = %q, want %q", env["FOO"], "bar")
	}
	if env["BAZ"] != "qux" {
		t.Errorf("BAZ = %q, want %q", env["BAZ"], "qux")
	}
}

func TestLoadFromDirNotExists(t *testing.T) {
	dir := t.TempDir()
	env, err := LoadFromDir(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if env != nil {
		t.Errorf("expected nil, got %v", env)
	}
}

func TestLoadFile(t *testing.T) {
	dir := t.TempDir()
	envFile := filepath.Join(dir, ".env")
	if err := os.WriteFile(envFile, []byte("A=1\nB=2"), 0644); err != nil {
		t.Fatalf("failed to write .env: %v", err)
	}
	env, err := LoadFile(envFile)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if env["A"] != "1" || env["B"] != "2" {
		t.Errorf("unexpected values: %v", env)
	}
}

func TestLoadFileNotExists(t *testing.T) {
	_, err := LoadFile("/nonexistent/.env")
	if err == nil {
		t.Error("expected error, got nil")
	}
}

func TestParseSingleQuotedPreservesHash(t *testing.T) {
	data := []byte("KEY='value # not a comment'")
	env, err := Parse(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if env["KEY"] != "value # not a comment" {
		t.Errorf("KEY = %q, want %q", env["KEY"], "value # not a comment")
	}
}
