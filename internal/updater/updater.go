// Package updater 从 GitHub Releases 检查并安装 Proton 的新版本。
// 仓库可能私有: 匿名访问会得到 404, 相关错误提示需要把设置令牌的方式讲清楚。
package updater

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"
)

const defaultAPIBase = "https://api.github.com"

// Options 控制一次更新检查/安装的行为。BaseURL 与 Executable 主要供测试注入,
// 留空时分别使用 https://api.github.com 和当前进程的二进制路径。
type Options struct {
	Repo       string       // 例如 "cyamoyed/proton"
	Token      string       // 可选; 私有仓库需要具有读取权限的令牌
	BaseURL    string       // GitHub API 地址, 默认 https://api.github.com
	Client     *http.Client // 默认带 5 分钟超时的客户端
	Executable string       // 被替换的二进制路径, 默认 os.Executable()
	Progress   func(string) // 用户可见的进度提示
}

// State 描述当前版本与最新发布的关系。
type State int

const (
	UpdateAvailable State = iota // 最新发布比当前版本新
	UpToDate                     // 版本一致
	CurrentNewer                 // 当前版本比最新发布新 (例如源码构建)
	UnknownCurrent               // 当前版本号无法解析 (例如 "dev")
)

// Release 是 GitHub Releases API 返回内容的所需子集。
type Release struct {
	TagName string  `json:"tag_name"`
	Assets  []Asset `json:"assets"`
}

// Asset 是发布附件的所需子集。URL 是 API 下载地址, 请求时带上
// Accept: application/octet-stream 与令牌即可下载 (含私有仓库)。
type Asset struct {
	Name string `json:"name"`
	URL  string `json:"url"`
}

// CheckResult 汇总一次版本检查的结果。
type CheckResult struct {
	Release *Release
	State   State
}

// Latest 返回最新发布的版本标签。
func (r CheckResult) Latest() string {
	if r.Release == nil {
		return ""
	}
	return r.Release.TagName
}

// Check 获取最新发布并与 currentVersion 比较。
func Check(ctx context.Context, opts Options, currentVersion string) (CheckResult, error) {
	release, err := FetchLatestRelease(ctx, opts)
	if err != nil {
		return CheckResult{}, err
	}
	state, err := compareState(currentVersion, release.TagName)
	if err != nil {
		return CheckResult{}, err
	}
	return CheckResult{Release: release, State: state}, nil
}

// FetchLatestRelease 从 GitHub Releases API 获取最新发布。
func FetchLatestRelease(ctx context.Context, opts Options) (*Release, error) {
	if opts.Repo == "" {
		return nil, fmt.Errorf("缺少仓库名")
	}
	base := opts.BaseURL
	if base == "" {
		base = defaultAPIBase
	}
	url := strings.TrimRight(base, "/") + "/repos/" + opts.Repo + "/releases/latest"
	req, err := opts.newRequest(ctx, http.MethodGet, url, "application/vnd.github+json")
	if err != nil {
		return nil, err
	}
	resp, err := opts.client().Do(req)
	if err != nil {
		return nil, fmt.Errorf("连接 GitHub 失败: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	switch resp.StatusCode {
	case http.StatusOK:
	case http.StatusNotFound:
		return nil, fmt.Errorf("GitHub 返回 404: 仓库 %s 不存在, 或为私有仓库且未设置令牌 (PROTON_GITHUB_TOKEN / GITHUB_TOKEN)", opts.Repo)
	case http.StatusUnauthorized:
		return nil, fmt.Errorf("GitHub 返回 401: 令牌无效, 请检查 PROTON_GITHUB_TOKEN / GITHUB_TOKEN")
	case http.StatusForbidden:
		return nil, fmt.Errorf("GitHub 返回 403: 令牌权限不足或 API 已限流, 请检查令牌权限或稍后重试")
	default:
		return nil, fmt.Errorf("GitHub 返回 HTTP %d", resp.StatusCode)
	}
	var release Release
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return nil, fmt.Errorf("解析最新版本信息: %w", err)
	}
	if release.TagName == "" {
		return nil, fmt.Errorf("最新版本信息中没有版本号")
	}
	return &release, nil
}

// AssetName 返回当前平台对应的发布附件名, 与 Makefile 的命名规则一致。
func AssetName() string {
	name := fmt.Sprintf("proton-%s-%s", runtime.GOOS, runtime.GOARCH)
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	return name
}

// Apply 下载 release 中与本机平台匹配的附件, 尽可能校验 SHA-256 后原子替换
// 当前二进制, 返回被更新的二进制路径。
func Apply(ctx context.Context, opts Options, release *Release) (string, error) {
	report := func(message string) {
		if opts.Progress != nil {
			opts.Progress(message)
		}
	}
	if release == nil {
		return "", fmt.Errorf("release 不能为空")
	}
	assetName := AssetName()
	asset, ok := findAsset(release, assetName)
	if !ok {
		return "", fmt.Errorf("版本 %s 中没有适配 %s/%s 的附件 %s", release.TagName, runtime.GOOS, runtime.GOARCH, assetName)
	}
	target, err := resolveExecutable(opts)
	if err != nil {
		return "", err
	}
	dir := filepath.Dir(target)
	staged, err := os.CreateTemp(dir, ".proton-update-")
	if err != nil {
		return "", fmt.Errorf("在 %s 创建临时文件: %w (系统目录可能需要管理员权限)", dir, err)
	}
	stagedPath := staged.Name()
	defer func() {
		_ = staged.Close()
		_ = os.Remove(stagedPath)
	}()

	report("下载 " + assetName + " ...")
	hasher := sha256.New()
	if err := download(ctx, opts, asset.URL, io.MultiWriter(staged, hasher)); err != nil {
		return "", fmt.Errorf("下载 %s: %w", assetName, err)
	}
	if err := staged.Close(); err != nil {
		return "", fmt.Errorf("写入临时文件: %w", err)
	}

	if expected, found := releaseChecksum(ctx, opts, release, assetName); found {
		if expected == "" {
			report("警告: checksums.txt 中没有 " + assetName + ", 跳过完整性校验")
		} else {
			report("校验 SHA-256 ...")
			actual := hex.EncodeToString(hasher.Sum(nil))
			if !strings.EqualFold(actual, expected) {
				return "", fmt.Errorf("SHA-256 校验失败, 下载的文件可能不完整或已被篡改")
			}
		}
	} else {
		report("警告: 无法获取 checksums.txt, 跳过完整性校验")
	}

	if err := os.Chmod(stagedPath, 0755); err != nil {
		return "", fmt.Errorf("设置可执行权限: %w", err)
	}
	report("更新 " + target + " ...")
	if err := replaceExecutable(stagedPath, target); err != nil {
		return "", err
	}
	return target, nil
}

// resolveExecutable 定位被替换的二进制: 优先使用注入路径, 否则取当前进程
// 的可执行文件, 并解析符号链接以更新真实文件。
func resolveExecutable(opts Options) (string, error) {
	path := opts.Executable
	if path == "" {
		executable, err := os.Executable()
		if err != nil {
			return "", fmt.Errorf("定位当前二进制: %w", err)
		}
		path = executable
	}
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		return "", fmt.Errorf("解析二进制路径 %s: %w", path, err)
	}
	return resolved, nil
}

// releaseChecksum 返回 assetName 在 checksums.txt 中的期望哈希。
// found 为 false 表示发布中没有 checksums.txt 或其下载失败;
// 空字符串表示文件存在但缺少该附件的条目。
func releaseChecksum(ctx context.Context, opts Options, release *Release, assetName string) (expected string, found bool) {
	asset, ok := findAsset(release, "checksums.txt")
	if !ok {
		return "", false
	}
	var sums bytes.Buffer
	if err := download(ctx, opts, asset.URL, &sums); err != nil {
		return "", false
	}
	return checksumFor(sums.Bytes(), assetName), true
}

// checksumFor 从 checksums.txt 内容中查找附件的 SHA-256 (兼容 "hash  name" 与 "hash *name")。
func checksumFor(sums []byte, assetName string) string {
	for _, line := range strings.Split(string(sums), "\n") {
		fields := strings.Fields(line)
		if len(fields) >= 2 && strings.TrimPrefix(fields[1], "*") == assetName {
			return fields[0]
		}
	}
	return ""
}

// replaceExecutable 用 staged 原子替换 target。Windows 不允许覆盖运行中的
// 二进制, 先把旧文件改名为 .old 再放入新文件; 遗留的 .old 由下次更新清理。
func replaceExecutable(staged, target string) error {
	if runtime.GOOS == "windows" {
		old := target + ".old"
		_ = os.Remove(old)
		if err := os.Rename(target, old); err != nil {
			return fmt.Errorf("备份当前二进制: %w", err)
		}
		if err := os.Rename(staged, target); err != nil {
			_ = os.Rename(old, target)
			return fmt.Errorf("激活新版本: %w", err)
		}
		return nil
	}
	if err := os.Rename(staged, target); err != nil {
		return fmt.Errorf("激活新版本: %w", err)
	}
	return nil
}

func findAsset(release *Release, name string) (Asset, bool) {
	for _, asset := range release.Assets {
		if asset.Name == name {
			return asset, true
		}
	}
	return Asset{}, false
}

func download(ctx context.Context, opts Options, url string, w io.Writer) error {
	req, err := opts.newRequest(ctx, http.MethodGet, url, "application/octet-stream")
	if err != nil {
		return err
	}
	resp, err := opts.client().Do(req)
	if err != nil {
		return fmt.Errorf("网络请求失败: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	if _, err := io.Copy(w, resp.Body); err != nil {
		return fmt.Errorf("读取内容失败: %w", err)
	}
	return nil
}

func (o Options) client() *http.Client {
	if o.Client != nil {
		return o.Client
	}
	return &http.Client{Timeout: 5 * time.Minute}
}

func (o Options) newRequest(ctx context.Context, method, url, accept string) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, method, url, nil)
	if err != nil {
		return nil, fmt.Errorf("构造请求: %w", err)
	}
	req.Header.Set("Accept", accept)
	req.Header.Set("User-Agent", "proton-updater")
	if o.Token != "" {
		req.Header.Set("Authorization", "Bearer "+o.Token)
	}
	return req, nil
}

var versionCorePattern = regexp.MustCompile(`^v?(\d+)\.(\d+)\.(\d+)`)

// ParseVersionCore 解析版本串开头的 X.Y.Z 三段。git describe 产生的
// "v1.2.3-4-gabcd" 以及不带 v 前缀的写法都支持; 无法解析时返回 false。
func ParseVersionCore(version string) ([3]int, bool) {
	match := versionCorePattern.FindStringSubmatch(strings.TrimSpace(version))
	if match == nil {
		return [3]int{}, false
	}
	var core [3]int
	for i := range core {
		value, err := strconv.Atoi(match[i+1])
		if err != nil {
			return [3]int{}, false
		}
		core[i] = value
	}
	return core, true
}

func compareState(current, latest string) (State, error) {
	currentCore, ok := ParseVersionCore(current)
	if !ok {
		return UnknownCurrent, nil
	}
	latestCore, ok := ParseVersionCore(latest)
	if !ok {
		return 0, fmt.Errorf("无法解析最新版本号 %q", latest)
	}
	for i := range currentCore {
		switch {
		case currentCore[i] < latestCore[i]:
			return UpdateAvailable, nil
		case currentCore[i] > latestCore[i]:
			return CurrentNewer, nil
		}
	}
	return UpToDate, nil
}
