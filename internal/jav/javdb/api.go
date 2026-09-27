package javdb

import (
	"context"
	"crypto/md5"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"javboss/internal/common/logging"
	"javboss/internal/jav/internal/parseutil"
	"javboss/internal/jav/internal/ratelimit"
	"javboss/internal/jav/metadata"
	"javboss/internal/util"
)

// App request protocol follows javdb-cli (MIT); see THIRD_PARTY_NOTICES.md.
const javDBAPIBaseURL = "https://jdforrepam.com"

var javDBAPIFC2CodeRe = regexp.MustCompile(`(?i)^FC2-(?:PPV-)?([0-9]+)$`)

// JavDB searches FC2 numbers without PPV. Keep all other separators significant.
func javDBAPIQueryCode(code string) string {
	code = strings.TrimSpace(code)
	if match := javDBAPIFC2CodeRe.FindStringSubmatch(code); match != nil {
		return "FC2-" + match[1]
	}
	return code
}

// API keeps its device identity and proxy-aware transport for the process lifetime.
type API struct {
	limiter  *ratelimit.Limiter
	once     sync.Once
	client   *http.Client
	baseURL  string
	deviceID string
}

func javDBAPISignature(ts int64) string {
	const prefix = "71cf27bb3c0bcdf207b64abecddc970098c7421ee7203b9cdae54478478a199e7d5a6e1a57691123c1a931c057842fb73ba3b3c83bcd69c17ccf174081e3d8aa"
	sum := md5.Sum([]byte(strconv.FormatInt(ts, 10) + prefix))
	return fmt.Sprintf("%d.lpw6vgqzsp.%x", ts, sum)
}

func (p *API) init() {
	p.once.Do(func() {
		if p.limiter == nil {
			p.limiter = ratelimit.New(javDBRequestInterval)
		}
		if p.client == nil {
			p.client = util.NewHTTPClient(20 * time.Second)
		}
		if p.baseURL == "" {
			p.baseURL = javDBAPIBaseURL
		}
		if p.deviceID == "" {
			var id [16]byte
			// crypto/rand.Read cannot fail on supported Go platforms.
			_, _ = rand.Read(id[:])
			id[6] = (id[6] & 0x0f) | 0x40
			id[8] = (id[8] & 0x3f) | 0x80
			p.deviceID = fmt.Sprintf("%x-%x-%x-%x-%x", id[:4], id[4:6], id[6:8], id[8:10], id[10:])
		}
	})
}

func (p *API) get(ctx context.Context, path string, params url.Values, dest any) (err error) {
	p.init()
	started := time.Now()
	status, responseBytes := 0, 0
	logging.Info("javdb-api request: url=%s%s code=%q", p.baseURL, path, params.Get("q"))
	defer func() {
		if err != nil && !errors.Is(err, metadata.ErrNotFound) {
			// url.Error includes the full query (including device_uuid); log only its cause.
			logErr := err
			var requestErr *url.Error
			if errors.As(err, &requestErr) {
				logErr = requestErr.Err
			}
			logging.Error("javdb-api request failed: path=%s status=%d bytes=%d elapsed=%s err=%v", path, status, responseBytes, time.Since(started).Round(time.Millisecond), logErr)
			return
		}
		logging.Info("javdb-api response: path=%s status=%d bytes=%d elapsed=%s", path, status, responseBytes, time.Since(started).Round(time.Millisecond))
	}()
	resp, err := p.request(ctx, path, params)
	if err != nil {
		return fmt.Errorf("javdb-api: request: %w", err)
	}
	defer resp.Body.Close()
	status = resp.StatusCode
	if resp.StatusCode == http.StatusNotFound {
		return metadata.ErrNotFound
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("javdb-api: http %d", resp.StatusCode)
	}
	const maxBody = 8 << 20
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxBody+1))
	responseBytes = len(body)
	if err != nil {
		return fmt.Errorf("javdb-api: read response: %w", err)
	}
	if len(body) > maxBody {
		return fmt.Errorf("javdb-api: response too large")
	}
	var envelope struct {
		Success json.RawMessage `json:"success"`
		Action  string          `json:"action"`
		Data    json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(body, &envelope); err != nil {
		return fmt.Errorf("javdb-api: decode response: %w", err)
	}
	success := strings.Trim(string(envelope.Success), "\"")
	if success != "1" && !strings.EqualFold(success, "true") {
		return fmt.Errorf("javdb-api: request rejected (%s)", envelope.Action)
	}
	if len(envelope.Data) == 0 || string(envelope.Data) == "null" {
		return fmt.Errorf("javdb-api: missing response data")
	}
	if err := json.Unmarshal(envelope.Data, dest); err != nil {
		return fmt.Errorf("javdb-api: decode data: %w", err)
	}
	return nil
}

// The API returns some IDs and numeric fields as either strings or numbers.
type javDBAPIValue string

func (v *javDBAPIValue) UnmarshalJSON(data []byte) error {
	if string(data) == "null" {
		*v = ""
		return nil
	}
	var s string
	if err := json.Unmarshal(data, &s); err == nil {
		*v = javDBAPIValue(strings.TrimSpace(s))
		return nil
	}
	var n json.Number
	if err := json.Unmarshal(data, &n); err != nil {
		return err
	}
	*v = javDBAPIValue(n.String())
	return nil
}

type javDBAPIMovie struct {
	ID          javDBAPIValue `json:"id"`
	Number      string        `json:"number"`
	Title       string        `json:"origin_title"`
	ZhTitle     string        `json:"title"`
	MakerName   string        `json:"maker_name"`
	MakerID     javDBAPIValue `json:"maker_id"`
	SeriesName  string        `json:"series_name"`
	SeriesID    javDBAPIValue `json:"series_id"`
	ReleaseDate string        `json:"release_date"`
	Duration    javDBAPIValue `json:"duration"`
	CoverURL    string        `json:"cover_url"`
	Type        javDBAPIValue `json:"type"`
	Tags        []struct {
		Name string `json:"name"`
	} `json:"tags"`
	Actors []struct {
		ID     javDBAPIValue `json:"id"`
		Name   string        `json:"name"`
		Gender javDBAPIValue `json:"gender"`
	} `json:"actors"`
	PreviewImages []struct {
		ThumbURL string `json:"thumb_url"`
		LargeURL string `json:"large_url"`
	} `json:"preview_images"`
}

// The API can localize title even when Accept-Language is Japanese.
// origin_title preserves the source title; older responses may omit it.
func (movie *javDBAPIMovie) metadataTitle() string {
	if title := strings.TrimSpace(movie.Title); title != "" {
		return title
	}
	return strings.TrimSpace(movie.ZhTitle)
}

func (p *API) movieByCode(ctx context.Context, code string) (*javDBAPIMovie, error) {
	code = javDBAPIQueryCode(code)
	if code == "" {
		return nil, metadata.ErrNotFound
	}
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	var search struct {
		Movies json.RawMessage `json:"movies"`
	}
	if err := p.get(ctx, "/api/v2/search", url.Values{"q": {code}, "page": {"1"}, "limit": {"100"}}, &search); err != nil {
		return nil, err
	}
	if len(search.Movies) == 0 {
		return nil, fmt.Errorf("javdb-api: missing search results")
	}
	var movies []javDBAPIMovie
	if err := json.Unmarshal(search.Movies, &movies); err != nil {
		return nil, fmt.Errorf("javdb-api: decode search results: %w", err)
	}
	id, err := resolveJavDBAPIMovieID(movies, code)
	if err != nil {
		logging.Info("javdb-api search unmatched: code=%q candidates=%d err=%v", code, len(movies), err)
		return nil, err
	}
	logging.Info("javdb-api search matched: code=%q movie_id=%s candidates=%d", code, id, len(movies))
	var data json.RawMessage
	if err := p.get(ctx, "/api/v4/movies/"+url.PathEscape(id), nil, &data); err != nil {
		return nil, err
	}
	var nested struct {
		Movie json.RawMessage `json:"movie"`
	}
	if err := json.Unmarshal(data, &nested); err != nil {
		return nil, fmt.Errorf("javdb-api: decode detail: %w", err)
	}
	if len(nested.Movie) != 0 {
		data = nested.Movie
	}
	var movie javDBAPIMovie
	if err := json.Unmarshal(data, &movie); err != nil {
		return nil, fmt.Errorf("javdb-api: decode movie: %w", err)
	}
	if !strings.EqualFold(javDBAPIQueryCode(movie.Number), code) || movie.metadataTitle() == "" {
		logging.Error("javdb-api invalid detail: code=%q movie_id=%s returned_code=%q has_title=%t", code, id, movie.Number, movie.metadataTitle() != "")
		return nil, fmt.Errorf("javdb-api: invalid or mismatched movie detail")
	}
	return &movie, nil
}

// Separators are part of the number: 053026_001 and 053026-001 must not match.
func resolveJavDBAPIMovieID(movies []javDBAPIMovie, code string) (string, error) {
	var id string
	for _, movie := range movies {
		if !strings.EqualFold(javDBAPIQueryCode(movie.Number), javDBAPIQueryCode(code)) {
			continue
		}
		if movie.ID == "" {
			return "", fmt.Errorf("javdb-api: matched movie has no id")
		}
		if id != "" && id != string(movie.ID) {
			return "", fmt.Errorf("javdb-api: ambiguous code %q", code)
		}
		id = string(movie.ID)
	}
	if id != "" {
		return id, nil
	}
	return "", metadata.ErrNotFound
}

func (p *API) LookupJavByCode(ctx context.Context, code string) (*metadata.JavInfo, error) {
	movie, err := p.movieByCode(ctx, code)
	if err != nil {
		return nil, err
	}
	info := &metadata.JavInfo{
		Title: movie.metadataTitle(), ZhTitle: strings.TrimSpace(movie.ZhTitle), Code: strings.TrimSpace(movie.Number),
		Studio: strings.TrimSpace(movie.MakerName), Series: strings.TrimSpace(movie.SeriesName),
		ReleaseUnix: parseutil.ParseDateUnix(movie.ReleaseDate), CoverURL: parseutil.ResolveSampleImageURL(p.baseURL, movie.CoverURL),
		Provider: metadata.ProviderJavDBAPI, SampleImages: []metadata.SampleImage{},
	}
	// Keep the application's canonical number stable for database linking and
	// subsequent metadata refreshes, regardless of the API's FC2 spelling.
	if match := javDBAPIFC2CodeRe.FindStringSubmatch(info.Code); match != nil {
		info.Code = "FC2-PPV-" + match[1]
	}
	info.DurationMin, _ = strconv.Atoi(string(movie.Duration))
	if info.DurationMin < 0 {
		info.DurationMin = 0
	}
	for _, tag := range movie.Tags {
		info.Tags = append(info.Tags, tag.Name)
	}
	for _, actor := range movie.Actors {
		if actor.Gender == "0" {
			info.Actors = append(info.Actors, actor.Name)
		}
	}
	info.Tags = parseutil.DedupeNonEmpty(info.Tags)
	info.Actors = parseutil.DedupeNonEmpty(info.Actors)
	seen := make(map[string]struct{})
	for _, sample := range movie.PreviewImages {
		thumbnail := parseutil.FirstResolvedSampleURL(p.baseURL, sample.ThumbURL, sample.LargeURL)
		detail := parseutil.FirstResolvedSampleURL(p.baseURL, sample.LargeURL, sample.ThumbURL)
		if detail == "" {
			continue
		}
		if _, exists := seen[detail]; exists {
			continue
		}
		seen[detail] = struct{}{}
		info.SampleImages = append(info.SampleImages, metadata.SampleImage{ThumbnailURL: thumbnail, DetailURL: detail})
	}
	// Detail responses use type (movie_type is a search parameter).
	// Only explicit censored/uncensored zones determine this field.
	if movie.Type == "0" || movie.Type == "1" {
		uncensored := movie.Type == "1"
		info.IsUncensored = &uncensored
	}
	return info, nil
}

func (p *API) LookupActressURLByCodeAndName(ctx context.Context, code, name string) (string, error) {
	if strings.TrimSpace(name) == "" {
		return "", metadata.ErrNotFound
	}
	movie, err := p.movieByCode(ctx, code)
	if err != nil {
		return "", err
	}
	for _, actor := range movie.Actors {
		if actor.Gender == "0" && strings.TrimSpace(actor.Name) == strings.TrimSpace(name) {
			return javDBAPIEntityURL("actors", actor.ID)
		}
	}
	return "", metadata.ErrNotFound
}
func (p *API) LookupSeriesURLByCode(ctx context.Context, code string) (string, error) {
	movie, err := p.movieByCode(ctx, code)
	if err != nil {
		return "", err
	}
	return javDBAPIEntityURL("series", movie.SeriesID)
}
func (p *API) LookupStudioURLByCode(ctx context.Context, code string) (string, error) {
	movie, err := p.movieByCode(ctx, code)
	if err != nil {
		return "", err
	}
	return javDBAPIEntityURL("makers", movie.MakerID)
}
func javDBAPIEntityURL(kind string, id javDBAPIValue) (string, error) {
	if id == "" {
		return "", metadata.ErrNotFound
	}
	return javDBBaseURL + "/" + kind + "/" + url.PathEscape(string(id)), nil
}

// NewAPI creates an app API client. Nil client and empty baseURL use production defaults.
func NewAPI(client *http.Client, baseURL string) *API { return &API{client: client, baseURL: baseURL} }

// NewProviders creates HTML and API clients sharing the site's request limit.
func NewProviders() (*Client, *API) {
	html := New()
	api := NewAPI(nil, "")
	api.limiter = html.limiter
	return html, api
}

// request shares API identity, signing, rate limiting and transport with connectivity checks.
func (p *API) request(ctx context.Context, path string, params url.Values) (*http.Response, error) {
	p.init()
	if err := p.limiter.Wait(ctx); err != nil {
		return nil, err
	}
	query := url.Values{
		"app_channel": {"official"}, "app_version": {"1.9.28"},
		"app_version_number": {"10928"}, "platform": {"android"},
		"system_version": {"13"}, "device_model": {"Pixel 6"},
		"device_name": {"Pixel"}, "device_uuid": {p.deviceID},
	}
	for key, values := range params {
		query[key] = values
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, p.baseURL+path+"?"+query.Encode(), nil)
	if err != nil {
		return nil, fmt.Errorf("javdb-api: build request: %w", err)
	}
	req.Header.Set("jdsignature", javDBAPISignature(time.Now().Unix()))
	req.Header.Set("User-Agent", "Dart/3.4 (dart:io)")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Accept-Language", "zh-TW")
	return p.client.Do(req)
}

// CheckConnectivity checks a signed search request without requiring a matching movie.
// The caller owns the response body.
func (p *API) CheckConnectivity(ctx context.Context) (*http.Response, error) {
	return p.request(ctx, "/api/v2/search", url.Values{"q": {"SSIS-001"}, "page": {"1"}, "limit": {"1"}})
}

// ConnectivityURL identifies the API origin, which is separate from the website.
func (p *API) ConnectivityURL() string {
	p.init()
	return p.baseURL
}
