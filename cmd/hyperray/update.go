// hyperray update: replace this binary with the newest release. Asks GitHub for
// the latest tag, skips the download when already current, and swaps the
// binary in place atomically so a failed download never breaks the install.
package main

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"

	"github.com/spf13/cobra"
)

const releaseAPI = "https://api.github.com/repos/HyperMarble/hyperray/releases/latest"

func newUpdateCmd() *cobra.Command {
	return &cobra.Command{
		Use:          "update",
		Short:        "Update hyperray to the latest release",
		Args:         cobra.NoArgs,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			out := cmd.OutOrStdout()
			latest, err := latestTag()
			if err != nil {
				return fmt.Errorf("update: %w", err)
			}
			if latest == version {
				fmt.Fprintf(out, "hyperray %s is already the latest\n", version)
				return nil
			}
			fmt.Fprintf(out, "updating %s -> %s\n", version, latest)

			url := fmt.Sprintf(
				"https://github.com/HyperMarble/hyperray/releases/download/%s/hyperray_%s_%s_%s.tar.gz",
				latest, latest, runtime.GOOS, runtime.GOARCH,
			)
			binary, err := downloadBinary(url)
			if err != nil {
				return fmt.Errorf("update: %w", err)
			}

			self, err := os.Executable()
			if err != nil {
				return err
			}
			// Write next to the real binary, then rename over it: rename on
			// the same filesystem is atomic, so an interrupted update never
			// leaves a half-written hyperray.
			staging := filepath.Join(filepath.Dir(self), ".hyperray-update")
			if err := os.WriteFile(staging, binary, 0o755); err != nil {
				return fmt.Errorf("update: %w", err)
			}
			if err := os.Rename(staging, self); err != nil {
				os.Remove(staging)
				return fmt.Errorf("update: %w", err)
			}
			fmt.Fprintf(out, "hyperray %s installed\n", latest)
			return nil
		},
	}
}

func latestTag() (string, error) {
	response, err := http.Get(releaseAPI)
	if err != nil {
		return "", err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return "", fmt.Errorf("release lookup returned %s", response.Status)
	}
	var release struct {
		TagName string `json:"tag_name"`
	}
	if err := json.NewDecoder(response.Body).Decode(&release); err != nil {
		return "", err
	}
	if release.TagName == "" {
		return "", fmt.Errorf("release lookup returned no tag")
	}
	return release.TagName, nil
}

func downloadBinary(url string) ([]byte, error) {
	response, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("download returned %s for %s", response.Status, url)
	}
	unzipped, err := gzip.NewReader(response.Body)
	if err != nil {
		return nil, err
	}
	archive := tar.NewReader(unzipped)
	for {
		header, err := archive.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		if filepath.Base(header.Name) == "hyperray" {
			return io.ReadAll(archive)
		}
	}
	return nil, fmt.Errorf("release archive holds no hyperray binary")
}
