package update

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"

	"golang.org/x/mod/semver"
)

const repository = "Solr159/JavBoss"

var releaseTag = regexp.MustCompile(`^v[0-9]+\.[0-9]+\.[0-9]+$`)

type asset struct {
	Name   string `json:"name"`
	URL    string `json:"browser_download_url"`
	Digest string `json:"digest"`
	Size   int64  `json:"size"`
}

type release struct {
	Tag        string  `json:"tag_name"`
	Notes      string  `json:"body"`
	Draft      bool    `json:"draft"`
	Prerelease bool    `json:"prerelease"`
	Assets     []asset `json:"assets"`
}

func platformName(goos, goarch string) string {
	return map[string]string{
		"windows/amd64": "windows-x86_64", "linux/amd64": "linux-x86_64",
		"darwin/amd64": "macos-x86_64", "darwin/arm64": "macos-arm64",
	}[goos+"/"+goarch]
}

// Extensions have their own releases in the same repository. Select application
// tags by semantic version, independently of GitHub's latest-release marker.
func latestRelease(ctx context.Context, client *http.Client, endpoint string) (*release, error) {
	var latest *release
	for page := 1; page <= 10; page++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("%s?per_page=100&page=%d", endpoint, page), nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("Accept", "application/vnd.github+json")
		req.Header.Set("User-Agent", "JavBoss-Updater")
		resp, err := client.Do(req)
		if err != nil {
			return nil, fmt.Errorf("check releases: %w", err)
		}
		if resp.StatusCode != http.StatusOK {
			resp.Body.Close()
			return nil, fmt.Errorf("check releases: HTTP %d", resp.StatusCode)
		}
		var releases []release
		err = json.NewDecoder(io.LimitReader(resp.Body, 8<<20)).Decode(&releases)
		resp.Body.Close()
		if err != nil {
			return nil, fmt.Errorf("decode releases: %w", err)
		}
		for _, r := range releases {
			if r.Draft || r.Prerelease || !releaseTag.MatchString(r.Tag) || !semver.IsValid(r.Tag) {
				continue
			}
			if latest == nil || semver.Compare(r.Tag, latest.Tag) > 0 {
				copy := r
				latest = &copy
			}
		}
		if len(releases) < 100 {
			return latest, nil
		}
	}
	return nil, fmt.Errorf("release history exceeds pagination limit")
}

func releaseAsset(r *release, platform string) (asset, error) {
	name := "javboss-" + r.Tag + "-" + platform + ".zip"
	for _, a := range r.Assets {
		if a.Name != name {
			continue
		}
		u, err := url.Parse(a.URL)
		if err != nil || u.Scheme != "https" || u.Host != "github.com" || u.User != nil ||
			u.Path != "/"+repository+"/releases/download/"+r.Tag+"/"+name || u.RawQuery != "" || u.Fragment != "" {
			return asset{}, fmt.Errorf("invalid release download URL")
		}
		digest, err := hex.DecodeString(strings.TrimPrefix(a.Digest, "sha256:"))
		if !strings.HasPrefix(a.Digest, "sha256:") || err != nil || len(digest) != 32 {
			return asset{}, fmt.Errorf("release asset has no valid SHA256 digest")
		}
		if a.Size <= 0 || a.Size > maxArchiveSize {
			return asset{}, fmt.Errorf("invalid release asset size")
		}
		return a, nil
	}
	return asset{}, fmt.Errorf("release has no package for %s", platform)
}
