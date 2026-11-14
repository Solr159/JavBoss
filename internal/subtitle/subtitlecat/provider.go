// Package subtitlecat implements the public HTML search/download flow. It does
// not invoke the site's translation endpoints or require a browser runtime.
package subtitlecat

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/PuerkitoBio/goquery"
	"javboss/internal/subtitle"
)

type Provider struct {
	client   *http.Client
	base     *url.URL
	mu       sync.Mutex
	next     time.Time
	interval time.Duration
}

func New(client *http.Client) *Provider {
	base, _ := url.Parse("https://www.subtitlecat.com")
	p := &Provider{base: base, interval: 500 * time.Millisecond}
	copy := *client
	copy.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		if len(via) >= 3 || !p.allowed(req.URL) {
			return fmt.Errorf("subtitlecat redirect outside source")
		}
		return nil
	}
	p.client = &copy
	return p
}

func (*Provider) ID() string { return "subtitlecat" }

func (p *Provider) allowed(u *url.URL) bool {
	return u.Scheme == p.base.Scheme && u.Host == p.base.Host && u.User == nil
}

func (p *Provider) address(ref string) (string, error) {
	u, err := url.Parse(strings.TrimSpace(ref))
	if err != nil {
		return "", err
	}
	u = p.base.ResolveReference(u)
	if !p.allowed(u) {
		return "", fmt.Errorf("subtitlecat link outside source")
	}
	return u.String(), nil
}

func (p *Provider) get(ctx context.Context, address string, limit int64) ([]byte, error) {
	u, err := p.address(address)
	if err != nil {
		return nil, err
	}
	p.mu.Lock()
	wait := time.Until(p.next)
	if wait < 0 {
		wait = 0
	}
	p.next = time.Now().Add(wait + p.interval)
	p.mu.Unlock()
	timer := time.NewTimer(wait)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-timer.C:
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0")
	req.Header.Set("Accept", "text/html,application/octet-stream;q=0.9,*/*;q=0.8")
	res, err := p.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("subtitlecat HTTP %d", res.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(res.Body, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > limit {
		return nil, fmt.Errorf("subtitlecat response too large")
	}
	return data, nil
}

// Match the whole code (including digit boundaries), tolerating separator and
// case differences. Never accept ABC-1234 when looking for ABC-123.
func codePattern(code string) *regexp.Regexp {
	parts := strings.FieldsFunc(code, func(r rune) bool { return r == '-' || r == '_' || r == ' ' })
	for i := range parts {
		parts[i] = regexp.QuoteMeta(parts[i])
	}
	return regexp.MustCompile(`(?i)(^|[^a-z0-9])` + strings.Join(parts, `[-_ ]?`) + `([^a-z0-9]|$)`)
}

var downloadCountPattern = regexp.MustCompile(`(?i)([0-9][0-9,]*)\s+downloads?\b`)

type rankedCandidate struct {
	subtitle.Candidate
	chineseSource bool
	goodRating    bool
	downloads     int64
}

func (p *Provider) Search(ctx context.Context, query subtitle.Query) ([]subtitle.Candidate, error) {
	u := *p.base
	u.Path = "/index.php"
	u.RawQuery = url.Values{"search": {query.Code}}.Encode()
	data, err := p.get(ctx, u.String(), 2<<20)
	if err != nil {
		return nil, err
	}
	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	var ranked []rankedCandidate
	seen := map[string]bool{}
	pattern := codePattern(query.Code)
	doc.Find("a[href]").Each(func(_ int, a *goquery.Selection) {
		href, _ := a.Attr("href")
		address, err := p.address(href)
		if err != nil {
			return
		}
		u, _ := url.Parse(address)
		title := strings.TrimSpace(a.Text())
		if !strings.HasPrefix(u.Path, "/subs/") || !strings.HasSuffix(u.Path, ".html") || !pattern.MatchString(title) || seen[address] {
			return
		}
		seen[address] = true
		row := a.Closest("tr")
		// Use the site's source annotation, not words in the uploaded title.
		sourceCell := a.Closest("td").Clone()
		sourceCell.Find("a").Remove()
		source := strings.ToLower(strings.Join(strings.Fields(sourceCell.Text()), " "))
		candidate := rankedCandidate{
			Candidate:     subtitle.Candidate{ID: address, Code: query.Code, Title: title},
			chineseSource: strings.Contains(source, "translated from chinese"),
			goodRating:    row.Find(`[title="Rated good by users"]`).Length() > 0 || strings.Contains(row.Find("td").Eq(1).Text(), "👍"),
		}
		row.Find("td:nth-child(n+3)").Each(func(_ int, cell *goquery.Selection) {
			if match := downloadCountPattern.FindStringSubmatch(cell.Text()); match != nil {
				if count, err := strconv.ParseInt(strings.ReplaceAll(match[1], ",", ""), 10, 64); err == nil {
					candidate.downloads = count
				}
			}
		})
		ranked = append(ranked, candidate)
	})
	// An unexpected document (e.g. a challenge page) is a transient failure,
	// not a missing subtitle eligible for the long negative-cache TTL.
	if doc.Find("input[name=search]").Length() == 0 {
		return nil, fmt.Errorf("subtitlecat search page changed or blocked")
	}
	// Rank before the coordinator limits detail lookups. Preserve page order
	// when metadata is missing or all three preferences tie.
	sort.SliceStable(ranked, func(i, j int) bool {
		a, b := ranked[i], ranked[j]
		if a.chineseSource != b.chineseSource {
			return a.chineseSource
		}
		if a.goodRating != b.goodRating {
			return a.goodRating
		}
		return a.downloads > b.downloads
	})
	candidates := make([]subtitle.Candidate, len(ranked))
	for i, candidate := range ranked {
		candidates[i] = candidate.Candidate
	}
	return candidates, nil
}

var languageSuffix = regexp.MustCompile(`(?i)-([a-z]{2,3}(?:-[a-z]{2,4})?)\.srt$`)

func (p *Provider) Tracks(ctx context.Context, c subtitle.Candidate) ([]subtitle.Track, error) {
	data, err := p.get(ctx, c.ID, 4<<20)
	if err != nil {
		return nil, err
	}
	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	var tracks []subtitle.Track
	doc.Find("a[href]").Each(func(_ int, a *goquery.Selection) {
		href, _ := a.Attr("href")
		address, err := p.address(href)
		if err != nil {
			return
		}
		u, _ := url.Parse(address)
		m := languageSuffix.FindStringSubmatch(u.Path)
		if !strings.HasPrefix(u.Path, "/subs/") || m == nil {
			return
		}
		lang := strings.ToLower(m[1])
		if lang == "zh-cn" {
			lang = "zh-CN"
		}
		if lang == "zh-tw" {
			lang = "zh-TW"
		}
		tracks = append(tracks, subtitle.Track{ID: address, Language: lang, Format: "srt"})
	})
	if len(tracks) == 0 && doc.Find("input[name=search]").Length() == 0 {
		return nil, fmt.Errorf("subtitlecat detail page changed or blocked")
	}
	return tracks, nil
}

func (p *Provider) Download(ctx context.Context, track subtitle.Track) ([]byte, error) {
	return p.get(ctx, track.ID, subtitle.MaxBytes)
}
