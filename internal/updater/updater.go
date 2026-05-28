package updater

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"time"

	"golang.org/x/sys/windows"
)

const userAgent = "dune-manager-updater/1.0"

type HelperPlan struct {
	WaitPID          int      `json:"waitPid"`
	SourcePath       string   `json:"sourcePath"`
	TargetPath       string   `json:"targetPath"`
	RestartPath      string   `json:"restartPath,omitempty"`
	RestartArgs      []string `json:"restartArgs,omitempty"`
	StartServiceName string   `json:"startServiceName,omitempty"`
	HideWindow       bool     `json:"hideWindow,omitempty"`
}

// Release is the GitHub Releases API response shape we care about.
type Release struct {
	TagName string  `json:"tag_name"`
	Assets  []Asset `json:"assets"`
}

// Asset is a single downloadable file in a release.
type Asset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
}

// UpdateInfo is returned by CheckForUpdate.
type UpdateInfo struct {
	Current   string
	Latest    string
	HasUpdate bool
	SvcURL    string // download URL for dune-manager-svc.exe
	GUIURL    string // download URL for dune-manager.exe
}

// CheckForUpdate queries the GitHub Releases API and returns update info.
// repo is "owner/repo" (e.g. "alice/dune-manager").
func CheckForUpdate(repo, currentVersion string) (*UpdateInfo, error) {
	rel, err := fetchLatestRelease(repo)
	if err != nil {
		return nil, err
	}

	info := &UpdateInfo{
		Current:   currentVersion,
		Latest:    rel.TagName,
		HasUpdate: rel.TagName != currentVersion && currentVersion != "dev",
	}

	for _, a := range rel.Assets {
		switch a.Name {
		case "dune-manager-svc.exe":
			info.SvcURL = a.BrowserDownloadURL
		case "dune-manager.exe":
			info.GUIURL = a.BrowserDownloadURL
		}
	}
	return info, nil
}

func fetchLatestRelease(repo string) (*Release, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/releases/latest", repo)
	client := &http.Client{Timeout: 15 * time.Second}
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Accept", "application/vnd.github+json")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("GitHub API: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == 404 {
		return nil, fmt.Errorf("no releases found for %s", repo)
	}
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("GitHub API returned HTTP %d", resp.StatusCode)
	}

	var r Release
	if err := json.NewDecoder(resp.Body).Decode(&r); err != nil {
		return nil, fmt.Errorf("parse release: %w", err)
	}
	return &r, nil
}

// DownloadToTemp downloads url into a temporary file and returns the path.
// The caller is responsible for removing the file when done.
func DownloadToTemp(url string, onProgress func(downloaded, total int64)) (string, error) {
	client := &http.Client{Timeout: 10 * time.Minute}
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("User-Agent", userAgent)

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return "", fmt.Errorf("download failed: HTTP %d", resp.StatusCode)
	}

	f, err := os.CreateTemp("", "dune-update-*.exe")
	if err != nil {
		return "", err
	}
	defer f.Close()

	total := resp.ContentLength
	reader := io.Reader(resp.Body)
	if onProgress != nil {
		reader = &progressReader{r: resp.Body, total: total, onProgress: onProgress}
	}

	if _, err := io.Copy(f, reader); err != nil {
		_ = os.Remove(f.Name())
		return "", err
	}
	return f.Name(), nil
}

// ApplyUpdate replaces targetPath with the binary at newBinaryPath.
//
// On Windows, the running executable can be renamed (but not deleted/overwritten
// directly) so we: rename old → .old, move/copy new → target.
// The .old file is cleaned up asynchronously after a short delay.
func ApplyUpdate(newBinaryPath, targetPath string) error {
	oldPath := targetPath + ".old"
	_ = os.Remove(oldPath)

	// Rename the current binary — works on Windows even while it is running.
	if err := os.Rename(targetPath, oldPath); err != nil {
		return fmt.Errorf("rename current binary: %w", err)
	}

	// Prefer a rename (same-drive, atomic); fall back to read+write.
	if err := os.Rename(newBinaryPath, targetPath); err != nil {
		data, readErr := os.ReadFile(newBinaryPath)
		if readErr != nil {
			_ = os.Rename(oldPath, targetPath) // restore
			return readErr
		}
		if writeErr := os.WriteFile(targetPath, data, 0755); writeErr != nil {
			_ = os.Rename(oldPath, targetPath) // restore
			return fmt.Errorf("write new binary: %w", writeErr)
		}
		_ = os.Remove(newBinaryPath)
	}

	go func() {
		time.Sleep(5 * time.Second)
		_ = os.Remove(oldPath)
	}()
	return nil
}

func HelperPath() string {
	if exe, err := os.Executable(); err == nil {
		return filepath.Join(filepath.Dir(exe), "dune-manager-updater.exe")
	}
	cwd, _ := os.Getwd()
	return filepath.Join(cwd, "dune-manager-updater.exe")
}

func LaunchHelper(plan HelperPlan) error {
	helperPath := HelperPath()
	if _, err := os.Stat(helperPath); err != nil {
		return fmt.Errorf("updater helper not found at %s", helperPath)
	}
	data, err := json.Marshal(plan)
	if err != nil {
		return fmt.Errorf("marshal helper plan: %w", err)
	}
	planFile, err := os.CreateTemp("", "dune-manager-update-plan-*.json")
	if err != nil {
		return fmt.Errorf("create helper plan: %w", err)
	}
	planPath := planFile.Name()
	if _, err := planFile.Write(data); err != nil {
		_ = planFile.Close()
		_ = os.Remove(planPath)
		return fmt.Errorf("write helper plan: %w", err)
	}
	if err := planFile.Close(); err != nil {
		_ = os.Remove(planPath)
		return fmt.Errorf("close helper plan: %w", err)
	}

	cmd := exec.Command(helperPath, "--plan", planPath)
	cmd.SysProcAttr = &syscall.SysProcAttr{
		HideWindow:    true,
		CreationFlags: uint32(windows.CREATE_NEW_PROCESS_GROUP | windows.DETACHED_PROCESS),
	}
	if err := cmd.Start(); err != nil {
		_ = os.Remove(planPath)
		return fmt.Errorf("start updater helper: %w", err)
	}
	return nil
}

// ── progress reader ────────────────────────────────────────────────────────────

type progressReader struct {
	r          io.Reader
	total      int64
	downloaded int64
	onProgress func(downloaded, total int64)
}

func (pr *progressReader) Read(p []byte) (int, error) {
	n, err := pr.r.Read(p)
	pr.downloaded += int64(n)
	if pr.onProgress != nil {
		pr.onProgress(pr.downloaded, pr.total)
	}
	return n, err
}
