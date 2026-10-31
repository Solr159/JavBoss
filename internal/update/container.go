package update

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strings"

	"golang.org/x/mod/semver"
)

const containerRepository = "solr159/javboss"

var registryNextLink = regexp.MustCompile(`<([^>]+)>;\s*rel="?next"?(?:\s|,|$)`)
var registryDigest = regexp.MustCompile(`^sha256:[a-f0-9]{64}$`)

const manifestAccept = "application/vnd.oci.image.index.v1+json, application/vnd.oci.image.manifest.v1+json, application/vnd.docker.distribution.manifest.list.v2+json, application/vnd.docker.distribution.manifest.v2+json"

// Docker releases are independent of desktop releases. Read the public registry
// anonymously and resolve the stable tag pointing to the same image as latest.
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
	manifestURL := endpoint + "/v2/" + containerRepository + "/manifests/"
	latestDigest, err := readRegistryManifestDigest(ctx, &registryClient, manifestURL+"latest", auth.Token)
	if err != nil {
		return nil, fmt.Errorf("resolve latest container image: %w", err)
	}
	base, err := url.Parse(endpoint + "/v2/" + containerRepository + "/tags/list?n=100")
	if err != nil {
		return nil, err
	}
	next := base.String()
	seen := make(map[string]bool)
	var tags []string
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
			if releaseTag.MatchString(tag) && semver.IsValid(tag) {
				tags = append(tags, tag)
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
	// Check newer tags first to avoid requesting every historical manifest. A
	// newer published version is not an update until latest points to that image.
	sort.Slice(tags, func(i, j int) bool { return semver.Compare(tags[i], tags[j]) > 0 })
	for _, tag := range tags {
		digest, err := readRegistryManifestDigest(ctx, &registryClient, manifestURL+tag, auth.Token)
		if err != nil {
			return nil, fmt.Errorf("resolve container image %s: %w", tag, err)
		}
		if digest == latestDigest {
			return &release{Tag: tag}, nil
		}
	}
	return nil, fmt.Errorf("latest container image has no matching stable version tag")
}

// Compare the complete multi-platform index, using identical content negotiation
// for latest and version tags. No image layers or platform archives are downloaded.
func readRegistryManifestDigest(ctx context.Context, client *http.Client, endpoint, token string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodHead, endpoint, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "JavBoss-Updater")
	req.Header.Set("Accept", manifestAccept)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("container manifest: HTTP %d", resp.StatusCode)
	}
	digest := resp.Header.Get("Docker-Content-Digest")
	if !registryDigest.MatchString(digest) {
		return "", fmt.Errorf("container manifest returned an invalid digest")
	}
	return digest, nil
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
