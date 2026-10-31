package update

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"

	"golang.org/x/mod/semver"
)

const containerRepository = "solr159/javboss"
const containerPackageURL = "https://github.com/" + repository + "/pkgs/container/javboss"

var registryNextLink = regexp.MustCompile(`<([^>]+)>;\s*rel="?next"?(?:\s|,|$)`)

// Docker releases are independent of desktop releases. Read the public registry
// anonymously and select the highest stable version, ignoring aliases and test tags.
func latestContainerRelease(ctx context.Context, client *http.Client, endpoint string) (*release, error) {
	// Registry metadata stays on this origin; do not forward the bearer token via redirects.
	registryClient := *client
	registryClient.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	var auth struct {
		Token       string `json:"token"`
		AccessToken string `json:"access_token"`
	}
	query := url.Values{"service": {"ghcr.io"}, "scope": {"repository:" + containerRepository + ":pull"}}
	if _, err := readRegistryJSON(ctx, &registryClient, endpoint+"/token?"+query.Encode(), "", &auth); err != nil {
		return nil, fmt.Errorf("get container registry token: %w", err)
	}
	if auth.Token == "" {
		auth.Token = auth.AccessToken
	}
	if auth.Token == "" {
		return nil, fmt.Errorf("container registry returned an empty token")
	}
	base, err := url.Parse(endpoint + "/v2/" + containerRepository + "/tags/list?n=100")
	if err != nil {
		return nil, err
	}
	next := base.String()
	seen := make(map[string]bool)
	var latest *release
	for next != "" {
		if seen[next] || len(seen) >= 100 {
			return nil, fmt.Errorf("container registry pagination did not complete")
		}
		seen[next] = true
		var page struct {
			Name string   `json:"name"`
			Tags []string `json:"tags"`
		}
		headers, err := readRegistryJSON(ctx, &registryClient, next, auth.Token, &page)
		if err != nil {
			return nil, fmt.Errorf("check container image versions: %w", err)
		}
		if page.Name != containerRepository {
			return nil, fmt.Errorf("container registry returned an unexpected repository")
		}
		for _, tag := range page.Tags {
			if releaseTag.MatchString(tag) && semver.IsValid(tag) && (latest == nil || semver.Compare(tag, latest.Tag) > 0) {
				latest = &release{Tag: tag}
			}
		}
		next = ""
		link := strings.Join(headers.Values("Link"), ",")
		if link == "" {
			break
		}
		match := registryNextLink.FindStringSubmatch(link)
		if match == nil {
			return nil, fmt.Errorf("container registry returned an invalid pagination link")
		}
		u, err := base.Parse(match[1])
		if err != nil || u.Scheme != base.Scheme || u.Host != base.Host || u.Path != base.Path || u.User != nil || u.Fragment != "" {
			return nil, fmt.Errorf("container registry returned an untrusted pagination link")
		}
		next = u.String()
	}
	return latest, nil
}

func readRegistryJSON(ctx context.Context, client *http.Client, endpoint, token string, result any) (http.Header, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "JavBoss-Updater")
	req.Header.Set("Accept", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("container registry: HTTP %d", resp.StatusCode)
	}
	const maxResponseSize = 1 << 20
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseSize+1))
	if err != nil {
		return nil, err
	}
	if len(data) > maxResponseSize {
		return nil, fmt.Errorf("container registry response is too large")
	}
	if err := json.Unmarshal(data, result); err != nil {
		return nil, fmt.Errorf("decode container registry response: %w", err)
	}
	return resp.Header, nil
}
