package download

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"gopkg.in/yaml.v3"
)

type Mirror struct {
	Name     string `yaml:"name" json:"name"`
	Type     string `yaml:"type" json:"type"`
	Template string `yaml:"template" json:"template"`
}

type MirrorsConfig struct {
	Mirrors []Mirror `yaml:"mirrors"`
}

type GithubRelease struct {
	TagName string `json:"tag_name"`
}

const FixedReleaseVersion = "0.68.1"
const fixedDownloadBaseURL = "http://8.162.14.1:8088/api/v1/alt-files/download"

type DownloadProgress struct {
	mu         sync.Mutex
	Total      int64
	Downloaded int64
	Percentage float64
	Done       bool
	Error      string
}

var ErrDownloadCanceled = errors.New("下载已停止")

func (dp *DownloadProgress) Reset() {
	dp.mu.Lock()
	defer dp.mu.Unlock()
	dp.Total = 0
	dp.Downloaded = 0
	dp.Percentage = 0
	dp.Done = false
	dp.Error = ""
}

func (dp *DownloadProgress) Update(downloaded, total int64) {
	dp.mu.Lock()
	defer dp.mu.Unlock()
	dp.Total = total
	dp.Downloaded = downloaded
	if total > 0 {
		dp.Percentage = float64(downloaded) / float64(total) * 100
	}
}

func (dp *DownloadProgress) Add(delta int64) {
	dp.mu.Lock()
	defer dp.mu.Unlock()
	dp.Downloaded += delta
	if dp.Total > 0 {
		dp.Percentage = float64(dp.Downloaded) / float64(dp.Total) * 100
	}
}

func (dp *DownloadProgress) SetTotal(total int64) {
	dp.mu.Lock()
	defer dp.mu.Unlock()
	dp.Total = total
	if total > 0 {
		dp.Percentage = float64(dp.Downloaded) / float64(total) * 100
	}
}

func (dp *DownloadProgress) SetDone(done bool) {
	dp.mu.Lock()
	defer dp.mu.Unlock()
	dp.Done = done
	if done && dp.Total > 0 && dp.Downloaded >= dp.Total {
		dp.Percentage = 100
	}
}

func (dp *DownloadProgress) SetError(message string) {
	dp.mu.Lock()
	defer dp.mu.Unlock()
	dp.Error = message
}

func (dp *DownloadProgress) GetPercentage() float64 {
	dp.mu.Lock()
	defer dp.mu.Unlock()
	return dp.Percentage
}

func (dp *DownloadProgress) IsDone() bool {
	dp.mu.Lock()
	defer dp.mu.Unlock()
	return dp.Done
}

func (dp *DownloadProgress) GetError() string {
	dp.mu.Lock()
	defer dp.mu.Unlock()
	return dp.Error
}

type ProgressWriter struct {
	Progress *DownloadProgress
}

func (pw *ProgressWriter) Write(p []byte) (int, error) {
	n := len(p)
	pw.Progress.Add(int64(n))
	return n, nil
}

var downloadProgress *DownloadProgress
var downloadProgressMu sync.Mutex
var activeDownloadCancel context.CancelFunc
var activeDownloadMu sync.Mutex

func GetDownloadProgress() *DownloadProgress {
	downloadProgressMu.Lock()
	defer downloadProgressMu.Unlock()
	if downloadProgress == nil {
		downloadProgress = &DownloadProgress{}
	}
	return downloadProgress
}

func StartDownloadContext() context.Context {
	activeDownloadMu.Lock()
	defer activeDownloadMu.Unlock()
	if activeDownloadCancel != nil {
		activeDownloadCancel()
	}
	ctx, cancel := context.WithCancel(context.Background())
	activeDownloadCancel = cancel
	return ctx
}

func FinishDownloadContext() {
	activeDownloadMu.Lock()
	defer activeDownloadMu.Unlock()
	activeDownloadCancel = nil
}

func CancelDownload() {
	activeDownloadMu.Lock()
	cancel := activeDownloadCancel
	activeDownloadCancel = nil
	activeDownloadMu.Unlock()
	if cancel != nil {
		cancel()
	}
}

func LoadMirrors(configPath string) ([]Mirror, error) {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, err
	}
	var config MirrorsConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, err
	}
	return config.Mirrors, nil
}

func FetchFrpVersions() ([]string, error) {
	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Get("https://api.github.com/repos/fatedier/frp/releases?per_page=10")
	if err != nil {
		return nil, fmt.Errorf("获取版本列表失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GitHub API 返回状态码 %d", resp.StatusCode)
	}

	var releases []GithubRelease
	if err := json.NewDecoder(resp.Body).Decode(&releases); err != nil {
		return nil, fmt.Errorf("解析版本列表失败: %w", err)
	}

	versions := make([]string, 0, len(releases))
	for _, r := range releases {
		versions = append(versions, r.TagName)
	}
	return versions, nil
}

func BuildDownloadURL(mirror Mirror, version, osName, arch string) string {
	versionNum := strings.TrimPrefix(version, "v")
	ext := ".tar.gz"
	if osName == "windows" {
		ext = ".zip"
	}
	filename := fmt.Sprintf("frp_%s_%s_%s%s", versionNum, osName, arch, ext)
	return BuildDownloadURLFromTemplate(mirror.Template, version, filename)
}

func BuildDownloadURLFromTemplate(template, version, filename string) string {
	template = strings.TrimSpace(template)
	if template == "" {
		template = "https://github.com/fatedier/frp/releases/download/{tag}/{filename}"
	}
	url := strings.ReplaceAll(template, "{tag}", version)
	url = strings.ReplaceAll(url, "{version}", strings.TrimPrefix(version, "v"))
	url = strings.ReplaceAll(url, "{filename}", filename)
	return url
}

func BuildFilename(version, osName, arch string) string {
	versionNum := strings.TrimPrefix(version, "v")
	ext := ".tar.gz"
	if osName == "windows" {
		ext = ".zip"
	}
	return fmt.Sprintf("frp_%s_%s_%s%s", versionNum, osName, arch, ext)
}

func BuildFixedFilename(osName, arch string) string {
	ext := ".tar.gz"
	if osName == "windows" {
		ext = ".zip"
	}
	return fmt.Sprintf("frp_%s_%s_%s%s", FixedReleaseVersion, osName, arch, ext)
}

func BuildFixedDownloadURL(filename string) string {
	return fixedDownloadBaseURL + "?path=" + url.QueryEscape(filename)
}

func DownloadFile(ctx context.Context, url, destPath string, progress *DownloadProgress) error {
	client := &http.Client{Timeout: 10 * time.Minute}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("创建下载请求失败: %w", err)
	}
	resp, err := client.Do(req)
	if err != nil {
		if errors.Is(err, context.Canceled) || ctx.Err() != nil {
			return ErrDownloadCanceled
		}
		return fmt.Errorf("下载失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("下载返回状态码 %d", resp.StatusCode)
	}

	out, err := os.Create(destPath)
	if err != nil {
		return fmt.Errorf("创建文件失败: %w", err)
	}
	defer out.Close()

	progress.SetTotal(resp.ContentLength)

	writer := io.MultiWriter(out, &ProgressWriter{
		Progress: progress,
	})

	_, err = io.Copy(writer, resp.Body)
	if err != nil {
		if errors.Is(err, context.Canceled) || ctx.Err() != nil {
			return ErrDownloadCanceled
		}
		return fmt.Errorf("下载写入失败: %w", err)
	}

	progress.SetDone(true)
	return nil
}

func ExtractFrpc(archivePath, destDir, osName string) error {
	if strings.HasSuffix(archivePath, ".zip") || osName == "windows" {
		return extractZip(archivePath, destDir)
	}
	return extractTarGz(archivePath, destDir)
}

func ExtractFrpcToFile(archivePath, destPath, osName string) error {
	if strings.HasSuffix(archivePath, ".zip") || osName == "windows" {
		return extractZipToFile(archivePath, destPath)
	}
	return extractTarGzToFile(archivePath, destPath)
}

func extractTarGz(archivePath, destDir string) error {
	return extractTarGzToFile(archivePath, filepath.Join(destDir, "frpc"))
}

func extractTarGzToFile(archivePath, destPath string) error {
	f, err := os.Open(archivePath)
	if err != nil {
		return err
	}
	defer f.Close()

	gzReader, err := gzip.NewReader(f)
	if err != nil {
		return err
	}
	defer gzReader.Close()

	tarReader := tar.NewReader(gzReader)

	if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
		return err
	}

	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		if strings.HasSuffix(header.Name, "frpc") || strings.HasSuffix(header.Name, "frpc.exe") {
			outFile, err := os.OpenFile(destPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0755)
			if err != nil {
				return err
			}
			defer outFile.Close()
			if _, err := io.Copy(outFile, tarReader); err != nil {
				return err
			}
			return nil
		}
	}
	return fmt.Errorf("压缩包中未找到 frpc")
}

func extractZip(archivePath, destDir string) error {
	return extractZipToFile(archivePath, filepath.Join(destDir, "frpc.exe"))
}

func extractZipToFile(archivePath, destPath string) error {
	reader, err := zip.OpenReader(archivePath)
	if err != nil {
		return err
	}
	defer reader.Close()

	if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
		return err
	}

	for _, file := range reader.File {
		if strings.HasSuffix(file.Name, "frpc.exe") || strings.HasSuffix(file.Name, "frpc") {
			rc, err := file.Open()
			if err != nil {
				return err
			}
			defer rc.Close()

			outFile, err := os.OpenFile(destPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0755)
			if err != nil {
				return err
			}
			defer outFile.Close()

			if _, err := io.Copy(outFile, rc); err != nil {
				return err
			}
			return nil
		}
	}
	return fmt.Errorf("压缩包中未找到 frpc.exe")
}

func CheckFrpcExists(toolsDir, platform string) bool {
	ext := ""
	if platform == "windows" || runtime.GOOS == "windows" {
		ext = ".exe"
	}
	for _, frpcPath := range []string{
		filepath.Join(toolsDir, "frpc"+ext),
		filepath.Join(toolsDir, platform, "frpc"+ext),
	} {
		if _, err := os.Stat(frpcPath); err == nil {
			return true
		}
	}
	return false
}

func ReadVersionFile(toolsDir string) string {
	versionPath := filepath.Join(toolsDir, "VERSION")
	data, err := os.ReadFile(versionPath)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

func WriteVersionFile(toolsDir, version string) error {
	versionPath := filepath.Join(toolsDir, "VERSION")
	return os.WriteFile(versionPath, []byte(version+"\n"), 0644)
}
