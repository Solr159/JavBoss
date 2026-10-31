package update

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strings"

	"golang.org/x/mod/semver"
)

const repository = "Solr159/JavBoss"

var releaseTag = regexp.MustCompile(`^v[0-9]+\.[0-9]+\.[0-9]+$`)

type asset struct {
	Name string
	URL  string
}

type release struct {
	Tag string
}

func platformName(goos, goarch string) string {
	return map[string]string{
		"windows/amd64": "windows-x86_64", "linux/amd64": "linux-x86_64",
		"darwin/amd64": "macos-x86_64", "darwin/arm64": "macos-arm64",
	}[goos+"/"+goarch]
}

// Read GitHub's Latest redirect without fetching its HTML or using the REST API.
// Fail closed if Latest was accidentally assigned to a browser extension release.
func latestRelease(ctx context.Context, client *http.Client, endpoint string) (*release, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodHead, endpoint, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "JavBoss-Updater")
	redirectClient := *client
	redirectClient.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	resp, err := redirectClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("check latest release: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return nil, nil
	}
	switch resp.StatusCode {
	case http.StatusMovedPermanently, http.StatusFound, http.StatusSeeOther, http.StatusTemporaryRedirect, http.StatusPermanentRedirect:
	default:
		return nil, fmt.Errorf("check latest release: expected a redirect, got HTTP %d", resp.StatusCode)
	}
	location, err := url.Parse(resp.Header.Get("Location"))
	if err != nil {
		return nil, fmt.Errorf("invalid latest release redirect: %w", err)
	}
	// GitHub may return a relative Location. Resolve against the trusted website,
	// not a caller-supplied Host or the redirected destination.
	base, _ := url.Parse("https://github.com/" + repository + "/releases/latest")
	location = base.ResolveReference(location)
	prefix := "/" + repository + "/releases/tag/"
	if location.Scheme != "https" || location.Host != "github.com" || location.User != nil ||
		location.RawQuery != "" || location.Fragment != "" || !strings.HasPrefix(location.Path, prefix) {
		return nil, fmt.Errorf("latest release does not point to an official application release")
	}
	tag := strings.TrimPrefix(location.Path, prefix)
	if !releaseTag.MatchString(tag) || !semver.IsValid(tag) {
		return nil, fmt.Errorf("latest release is not a stable application version: %s", tag)
	}
	return &release{Tag: tag}, nil
}

// Release archives have fixed names, so no API or manifest is needed.
func releaseAsset(r *release, platform string) (asset, error) {
	if r == nil || !releaseTag.MatchString(r.Tag) || !semver.IsValid(r.Tag) {
		return asset{}, fmt.Errorf("invalid application release version")
	}
	switch platform {
	case "windows-x86_64", "linux-x86_64", "macos-x86_64", "macos-arm64":
	default:
		return asset{}, fmt.Errorf("unsupported release platform: %s", platform)
	}
	name := "javboss-" + r.Tag + "-" + platform + ".zip"
	return asset{Name: name, URL: "https://github.com/" + repository + "/releases/download/" + r.Tag + "/" + name}, nil
}
