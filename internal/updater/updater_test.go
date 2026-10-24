package updater

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// fakeRelease 描述测试服务器上发布的内容: 附件名到内容的映射。
type fakeRelease struct {
	Tag    string
	Assets map[string]string
}

// newReleaseServer 启动一个模拟 GitHub Releases API 的测试服务器。
// status 非 0 时 API 一律返回该状态码。
func newReleaseServer(t *testing.T, release fakeRelease, status int) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/repos/o/r/releases/latest", func(w http.ResponseWriter, _ *http.Request) {
		if status != 0 {
			w.WriteHeader(status)
			return
		}
		type asset struct {
			Name string `json:"name"`
			URL  string `json:"url"`
		}
		assets := make([]asset, 0, len(release.Assets))
		for name := range release.Assets {
			assets = append(assets, asset{Name: name, URL: "/repos/o/r/download/" + name})
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"tag_name": release.Tag, "assets": assets})
	})
	for name, content := range release.Assets {
		mux.HandleFunc("/repos/o/r/download/"+name, func(w http.ResponseWriter, _ *http.Request) {
			fmt.Fprint(w, content)
		})
	}
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)
	return server
}

// serverRelease 把 fakeRelease 转成指向测试服务器的 Release。
func serverRelease(server *httptest.Server, release fakeRelease) *Release {
	result := &Release{TagName: release.Tag}
	for name := range release.Assets {
		result.Assets = append(result.Assets, Asset{Name: name, URL: server.URL + "/repos/o/r/download/" + name})
	}
	return result
}

// releaseWithChecksum 构造带真实 SHA-256 的发布, checksums.txt 校验 binary。
func releaseWithChecksum(t *testing.T, tag, binary string) fakeRelease {
	t.Helper()
	sum := sha256.Sum256([]byte(binary))
	return fakeRelease{Tag: tag, Assets: map[string]string{
		AssetName():     binary,
		"checksums.txt": hex.EncodeToString(sum[:]) + "  " + AssetName() + "\n",
	}}
}

func testOptions(server *httptest.Server, target string) Options {
	return Options{
		Repo:       "o/r",
		BaseURL:    server.URL,
		Executable: target,
		Progress:   func(string) {},
	}
}

func TestParseVersionCore(t *testing.T) {
	cases := []struct {
		version string
		want    [3]int
		ok      bool
	}{
		{"v1.2.3", [3]int{1, 2, 3}, true},
		{"1.2.3", [3]int{1, 2, 3}, true},
		{"v0.10.20", [3]int{0, 10, 20}, true},
		{"v1.2.3-4-gabcd123", [3]int{1, 2, 3}, true},
		{" v1.2.3 ", [3]int{1, 2, 3}, true},
		{"dev", [3]int{}, false},
		{"", [3]int{}, false},
		{"v1.2", [3]int{}, false},
		{"abc", [3]int{}, false},
	}
	for _, tc := range cases {
		got, ok := ParseVersionCore(tc.version)
		if ok != tc.ok || got != tc.want {
			t.Errorf("ParseVersionCore(%q) = %v, %v; want %v, %v", tc.version, got, ok, tc.want, tc.ok)
		}
	}
}

func TestCompareState(t *testing.T) {
	cases := []struct {
		current, latest string
		want            State
	}{
		{"v1.2.3", "v1.2.4", UpdateAvailable},
		{"v1.2.3", "v1.3.0", UpdateAvailable},
		{"v1.2.3", "v2.0.0", UpdateAvailable},
		{"v1.2.3", "v1.2.3", UpToDate},
		{"v1.2.3", "1.2.3", UpToDate},
		{"v1.2.3-4-gabcd", "v1.2.3", UpToDate},
		{"v1.2.4", "v1.2.3", CurrentNewer},
		{"dev", "v1.2.3", UnknownCurrent},
	}
	for _, tc := range cases {
		got, err := compareState(tc.current, tc.latest)
		if err != nil {
			t.Errorf("compareState(%q, %q) error = %v", tc.current, tc.latest, err)
			continue
		}
		if got != tc.want {
			t.Errorf("compareState(%q, %q) = %v; want %v", tc.current, tc.latest, got, tc.want)
		}
	}
	if _, err := compareState("v1.2.3", "release-2024"); err == nil {
		t.Error("unparsable latest version should fail")
	}
}

func TestCheckAgainstServer(t *testing.T) {
	cases := []struct {
		name, current string
		want          State
	}{
		{"older current gets update", "v1.0.0", UpdateAvailable},
		{"equal version is up to date", "v2.0.0", UpToDate},
		{"newer current", "v2.1.0", CurrentNewer},
		{"dev build is unknown", "dev", UnknownCurrent},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			server := newReleaseServer(t, releaseWithChecksum(t, "v2.0.0", "binary"), 0)
			result, err := Check(context.Background(), testOptions(server, ""), tc.current)
			if err != nil {
				t.Fatalf("Check() error = %v", err)
			}
			if result.State != tc.want {
				t.Errorf("State = %v; want %v", result.State, tc.want)
			}
			if result.Latest() != "v2.0.0" {
				t.Errorf("Latest() = %q; want v2.0.0", result.Latest())
			}
		})
	}
}

func TestCheckErrorStatuses(t *testing.T) {
	cases := []struct {
		status int
		hint   string
	}{
		{http.StatusNotFound, "令牌"},
		{http.StatusUnauthorized, "令牌"},
		{http.StatusForbidden, "限流"},
	}
	for _, tc := range cases {
		t.Run(fmt.Sprintf("HTTP %d", tc.status), func(t *testing.T) {
			server := newReleaseServer(t, fakeRelease{}, tc.status)
			_, err := Check(context.Background(), testOptions(server, ""), "v1.0.0")
			if err == nil {
				t.Fatal("expected an error")
			}
			if !strings.Contains(err.Error(), tc.hint) {
				t.Errorf("error %q should mention %q", err, tc.hint)
			}
		})
	}
}

func TestApplyReplacesExecutable(t *testing.T) {
	release := releaseWithChecksum(t, "v2.0.0", "new binary")
	server := newReleaseServer(t, release, 0)
	dir := t.TempDir()
	target := filepath.Join(dir, "proton")
	if err := os.WriteFile(target, []byte("old binary"), 0755); err != nil {
		t.Fatal(err)
	}

	applied, err := Apply(context.Background(), testOptions(server, target), serverRelease(server, release))
	if err != nil {
		t.Fatalf("Apply() error = %v", err)
	}
	// Apply 会先 EvalSymlinks 再替换, 断言要用解析后的真实路径 (macOS 的
	// /var 是 /private/var 的符号链接)。
	resolvedTarget, err := filepath.EvalSymlinks(target)
	if err != nil {
		t.Fatal(err)
	}
	if applied != resolvedTarget {
		t.Errorf("Apply() = %q; want %q", applied, resolvedTarget)
	}
	data, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "new binary" {
		t.Errorf("target content = %q; want %q", data, "new binary")
	}
	info, err := os.Stat(target)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm()&0111 == 0 {
		t.Errorf("target mode %v is not executable", info.Mode().Perm())
	}
	assertNoStagedFiles(t, dir)
}

func TestApplyChecksumMismatchKeepsOldBinary(t *testing.T) {
	release := fakeRelease{Tag: "v2.0.0", Assets: map[string]string{
		AssetName():     "tampered binary",
		"checksums.txt": "0000000000000000000000000000000000000000000000000000000000000000  " + AssetName() + "\n",
	}}
	server := newReleaseServer(t, release, 0)
	dir := t.TempDir()
	target := filepath.Join(dir, "proton")
	if err := os.WriteFile(target, []byte("old binary"), 0755); err != nil {
		t.Fatal(err)
	}

	_, err := Apply(context.Background(), testOptions(server, target), serverRelease(server, release))
	if err == nil || !strings.Contains(err.Error(), "SHA-256") {
		t.Fatalf("Apply() error = %v; want SHA-256 mismatch", err)
	}
	data, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "old binary" {
		t.Errorf("target content = %q; the old binary must survive a failed update", data)
	}
	assertNoStagedFiles(t, dir)
}

func TestApplyWithoutChecksumsStillUpdates(t *testing.T) {
	release := fakeRelease{Tag: "v2.0.0", Assets: map[string]string{AssetName(): "new binary"}}
	server := newReleaseServer(t, release, 0)
	dir := t.TempDir()
	target := filepath.Join(dir, "proton")
	if err := os.WriteFile(target, []byte("old binary"), 0755); err != nil {
		t.Fatal(err)
	}

	if _, err := Apply(context.Background(), testOptions(server, target), serverRelease(server, release)); err != nil {
		t.Fatalf("Apply() error = %v", err)
	}
	data, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "new binary" {
		t.Errorf("target content = %q; want %q", data, "new binary")
	}
}

func TestApplyMissingPlatformAsset(t *testing.T) {
	release := fakeRelease{Tag: "v2.0.0", Assets: map[string]string{"proton-other-os-arm64": "binary"}}
	server := newReleaseServer(t, release, 0)
	dir := t.TempDir()

	_, err := Apply(context.Background(), testOptions(server, filepath.Join(dir, "proton")), serverRelease(server, release))
	if err == nil || !strings.Contains(err.Error(), AssetName()) {
		t.Fatalf("Apply() error = %v; want it to mention the missing asset %s", err, AssetName())
	}
	assertNoStagedFiles(t, dir)
}

func TestApplyDownloadFailure(t *testing.T) {
	release := releaseWithChecksum(t, "v2.0.0", "new binary")
	server := newReleaseServer(t, release, 0)
	// 附件 URL 指向不存在的路径, 模拟下载失败。
	broken := &Release{TagName: release.Tag, Assets: []Asset{{Name: AssetName(), URL: server.URL + "/missing"}}}
	dir := t.TempDir()
	target := filepath.Join(dir, "proton")
	if err := os.WriteFile(target, []byte("old binary"), 0755); err != nil {
		t.Fatal(err)
	}

	_, err := Apply(context.Background(), testOptions(server, target), broken)
	if err == nil || !strings.Contains(err.Error(), "下载") {
		t.Fatalf("Apply() error = %v; want a download failure", err)
	}
	data, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "old binary" {
		t.Errorf("target content = %q; the old binary must survive a failed update", data)
	}
	assertNoStagedFiles(t, dir)
}

func TestAssetName(t *testing.T) {
	name := AssetName()
	suffix := ""
	if runtime.GOOS == "windows" {
		suffix = ".exe"
	}
	want := fmt.Sprintf("proton-%s-%s%s", runtime.GOOS, runtime.GOARCH, suffix)
	if name != want {
		t.Errorf("AssetName() = %q; want %q", name, want)
	}
}

func TestChecksumFor(t *testing.T) {
	sums := "aaaaaaaa  proton-linux-amd64\nbbbbbbbb *proton-darwin-arm64\n"
	if got := checksumFor([]byte(sums), "proton-darwin-arm64"); got != "bbbbbbbb" {
		t.Errorf("checksumFor() = %q; want bbbbbbbb", got)
	}
	if got := checksumFor([]byte(sums), "proton-linux-amd64"); got != "aaaaaaaa" {
		t.Errorf("checksumFor() = %q; want aaaaaaaa", got)
	}
	if got := checksumFor([]byte(sums), "missing"); got != "" {
		t.Errorf("checksumFor() = %q; want empty", got)
	}
}

// assertNoStagedFiles 确认目录中没有残留的 .proton-update-* 临时文件。
func assertNoStagedFiles(t *testing.T, dir string) {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), ".proton-update-") {
			t.Errorf("staged file %s was left behind", entry.Name())
		}
	}
}
