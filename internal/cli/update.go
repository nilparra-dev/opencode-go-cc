// Package cli provides the command-line interface for occb.
package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

const (
	updateRepoOwner = "nilparra-dev"
	updateRepoName  = "opencode-go-cc"
	updateBinary    = "occb"
)

type releaseInfo struct {
	TagName string         `json:"tag_name"`
	Assets  []releaseAsset `json:"assets"`
}

type releaseAsset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
}

// NewUpdateCmd creates the self-update command.
func NewUpdateCmd(currentVersion string) *cobra.Command {
	var checkOnly bool
	var force bool

	cmd := &cobra.Command{
		Use:     "update",
		Aliases: []string{"upgrade", "self-update"},
		Short:   "Download and install the latest occb release",
		RunE: func(cmd *cobra.Command, args []string) error {
			release, asset, releaseErr := fetchLatestRelease(cmd.Context())

			if checkOnly {
				if releaseErr != nil {
					if canBuildFromSource() {
						fmt.Println("Latest GitHub release metadata is unavailable; `occb update` will fall back to the latest Go module source.")
						return nil
					}
					return releaseErr
				}
				if versionMatches(currentVersion, release.TagName) {
					fmt.Printf("occb %s is already up to date\n", currentVersion)
				} else {
					fmt.Printf("Latest release: %s\n", release.TagName)
				}
				return nil
			}

			if releaseErr == nil && !force && versionMatches(currentVersion, release.TagName) {
				fmt.Printf("occb %s is already up to date\n", currentVersion)
				return nil
			}

			targetPath, err := updateTargetPath()
			if err != nil {
				return err
			}

			stagePath := targetPath + ".new"
			updatedFromSource := false
			updatedVersion := "latest source"

			if releaseErr == nil {
				updatedVersion = release.TagName
				if err := downloadReleaseAsset(cmd.Context(), asset.BrowserDownloadURL, stagePath); err != nil {
					releaseErr = err
				}
			}

			if releaseErr != nil {
				if err := buildLatestFromSource(cmd.Context(), stagePath); err != nil {
					return fmt.Errorf("release update failed: %v; source fallback failed: %w", releaseErr, err)
				}
				updatedFromSource = true
			}

			if err := applyReleaseUpdate(stagePath, targetPath); err != nil {
				_ = os.Remove(stagePath)
				return err
			}

			if updatedFromSource {
				fmt.Println("Updated occb from the latest Go module source")
			} else {
				fmt.Printf("Updated occb to %s\n", updatedVersion)
			}
			if runtime.GOOS == "windows" {
				fmt.Println("The new binary has been staged in place. Open a new terminal before using occb again.")
			} else {
				fmt.Println("Re-run occb to use the updated binary.")
			}

			return nil
		},
	}

	cmd.Flags().BoolVar(&checkOnly, "check", false, "Only check whether a newer release is available")
	cmd.Flags().BoolVar(&force, "force", false, "Install the latest release even if the version matches")

	return cmd
}

func fetchLatestRelease(ctx context.Context) (*releaseInfo, *releaseAsset, error) {
	assetName, err := platformAssetName(runtime.GOOS, runtime.GOARCH)
	if err != nil {
		return nil, nil, err
	}

	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/releases/latest", updateRepoOwner, updateRepoName)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to build GitHub request: %w", err)
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "occb-updater")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to fetch latest release: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return nil, nil, fmt.Errorf("GitHub release lookup failed: %s: %s", resp.Status, strings.TrimSpace(string(body)))
	}

	var release releaseInfo
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return nil, nil, fmt.Errorf("failed to decode latest release: %w", err)
	}

	for _, asset := range release.Assets {
		if asset.Name == assetName {
			return &release, &asset, nil
		}
	}

	return nil, nil, fmt.Errorf("latest release %s does not contain an asset for %s", release.TagName, assetName)
}

func platformAssetName(goos, goarch string) (string, error) {
	switch goos {
	case "windows", "linux", "darwin":
	default:
		return "", fmt.Errorf("unsupported OS for self-update: %s", goos)
	}

	switch goarch {
	case "amd64", "arm64":
	default:
		return "", fmt.Errorf("unsupported architecture for self-update: %s", goarch)
	}

	name := fmt.Sprintf("%s_%s-%s", updateBinary, goos, goarch)
	if goos == "windows" {
		name += ".exe"
	}

	return name, nil
}

func downloadReleaseAsset(ctx context.Context, url, destination string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("failed to build asset request: %w", err)
	}
	req.Header.Set("User-Agent", "occb-updater")

	client := &http.Client{Timeout: 2 * time.Minute}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to download release asset: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return fmt.Errorf("asset download failed: %s: %s", resp.Status, strings.TrimSpace(string(body)))
	}

	if err := os.MkdirAll(filepath.Dir(destination), 0755); err != nil {
		return fmt.Errorf("failed to create update directory: %w", err)
	}

	file, err := os.OpenFile(destination, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0755)
	if err != nil {
		return fmt.Errorf("failed to create staging file: %w", err)
	}
	defer file.Close()

	if _, err := io.Copy(file, resp.Body); err != nil {
		return fmt.Errorf("failed to write staging file: %w", err)
	}

	return file.Close()
}

func buildLatestFromSource(ctx context.Context, destination string) error {
	goExe, err := exec.LookPath("go")
	if err != nil {
		return fmt.Errorf("go is required for source fallback but was not found in PATH")
	}

	tempDir, err := os.MkdirTemp("", "occb-update-build-")
	if err != nil {
		return fmt.Errorf("failed to create Go build temp dir: %w", err)
	}
	defer os.RemoveAll(tempDir)

	cmd := exec.CommandContext(ctx, goExe, "install", fmt.Sprintf("github.com/%s/%s/cmd/%s@latest", updateRepoOwner, updateRepoName, updateBinary))
	cmd.Env = append(os.Environ(), "GOBIN="+tempDir)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("go install failed: %w: %s", err, strings.TrimSpace(string(output)))
	}

	builtBinary := filepath.Join(tempDir, updateBinary)
	if runtime.GOOS == "windows" {
		builtBinary += ".exe"
	}

	data, err := os.ReadFile(builtBinary)
	if err != nil {
		return fmt.Errorf("failed to read built binary: %w", err)
	}

	if err := os.WriteFile(destination, data, 0755); err != nil {
		return fmt.Errorf("failed to stage built binary: %w", err)
	}

	return nil
}

func canBuildFromSource() bool {
	_, err := exec.LookPath("go")
	return err == nil
}

func updateTargetPath() (string, error) {
	exePath, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("failed to resolve executable path: %w", err)
	}

	resolvedPath, err := filepath.EvalSymlinks(exePath)
	if err == nil {
		return resolvedPath, nil
	}

	return exePath, nil
}

func versionMatches(currentVersion, latestVersion string) bool {
	if currentVersion == "" || currentVersion == "dev" || latestVersion == "" {
		return false
	}

	trimmedCurrent := strings.TrimPrefix(currentVersion, "v")
	trimmedLatest := strings.TrimPrefix(latestVersion, "v")
	return trimmedCurrent == trimmedLatest
}