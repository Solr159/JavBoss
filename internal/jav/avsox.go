package jav

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"golang.org/x/net/html"

	"javboss/internal/common/logging"
	"javboss/internal/util"
)

// avsox implements lookupProvider.
type avsox struct{}

var avsoxProvider lookupProvider = avsox{}

const (
	avsoxBaseURL         = "https://avsox.click"
	avsoxUserAgent       = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"
	avsoxRequestInterval = 1500 * time.Millisecond
	avsoxAPILanguage     = "cn"
	avsoxAPISearchLimit  = 60
	avsoxLookupTimeout   = 90 * time.Second
	avsoxHTTPTimeout     = 30 * time.Second
	avsoxAPITries        = 3
	avsoxAPIRetryDelay   = 2 * time.Second
	avsoxSessionTTL      = 30 * time.Minute
)

var avsoxRateLimiter = struct {
	sync.Mutex
	next time.Time
}{}

var (
	avsoxHTTPClientOnce sync.Once
	avsoxHTTPClient     *http.Client
)

var avsoxSessionCache = struct {
	sync.Mutex
	session   avsoxSession
	expiresAt time.Time
}{}

type avsoxSession struct {
	csrfToken string
	cookie    string
	referer   string
}

type avsoxAPIEnvelope struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data"`
}

type avsoxStatusError struct {
	source  string
	status  int
	message string
}

func (e avsoxStatusError) Error() string {
	if strings.TrimSpace(e.message) != "" {
		return fmt.Sprintf("avsox: %s code %d: %s", e.source, e.status, e.message)
	}
	return fmt.Sprintf("avsox: %s code %d", e.source, e.status)
}

type avsoxAPIMovie struct {
	MovieID     string          `json:"movieId"`
	MovieFanHao string          `json:"movieFanHao"`
	Title       string          `json:"title"`
	TitleJA     string          `json:"title_ja"`
	TitleEN     string          `json:"title_en"`
	TitleCN     string          `json:"title_cn"`
	TitleTW     string          `json:"title_tw"`
	ReleaseDate string          `json:"releaseDate"`
	Length      int             `json:"length"`
	PosterSmall string          `json:"posterSmall"`
	PosterLarge string          `json:"posterLarge"`
	Studio      *avsoxAPIStudio `json:"studio"`
	Series      *avsoxAPISeries `json:"series"`
	Genre       []avsoxAPIGenre `json:"genre"`
	Star        []avsoxAPIStar  `json:"star"`
}

type avsoxAPIStudio struct {
	StudioName   string `json:"studioName"`
	StudioNameJA string `json:"studioName_ja"`
	StudioNameEN string `json:"studioName_en"`
	StudioNameCN string `json:"studioName_cn"`
	StudioNameTW string `json:"studioName_tw"`
}

type avsoxAPISeries struct {
	SeriesName   string `json:"seriesName"`
	SeriesNameJA string `json:"seriesName_ja"`
	SeriesNameEN string `json:"seriesName_en"`
	SeriesNameCN string `json:"seriesName_cn"`
	SeriesNameTW string `json:"seriesName_tw"`
}

type avsoxAPIGenre struct {
	GenreName   string `json:"genreName"`
	GenreNameJA string `json:"genreName_ja"`
	GenreNameEN string `json:"genreName_en"`
	GenreNameCN string `json:"genreName_cn"`
	GenreNameTW string `json:"genreName_tw"`
}

type avsoxAPIStar struct {
	StarName   string `json:"starName"`
	StarNameJA string `json:"starName_ja"`
	StarNameEN string `json:"starName_en"`
	StarNameCN string `json:"starName_cn"`
	StarNameTW string `json:"starName_tw"`
}

// LookupActressByName implements lookupProvider.
func (avsox) LookupActressByName(name string) (*ActressInfo, error) {
	return nil, errors.New("avsox: lookup actress not supported")
}

// LookupActressByCode implements lookupProvider.
func (avsox) LookupActressByCode(code string) (*ActressInfo, error) {
	return nil, errors.New("avsox: lookup actress not supported")
}

// LookupActressURLByCodeAndName implements lookupProvider.
func (avsox) LookupActressURLByCodeAndName(code, name string) (string, error) {
	return "", errors.New("avsox: lookup actress url not supported")
}

// LookupCoverURLByCode resolves a cover image URL for a movie code.
func (avsox) LookupCoverURLByCode(code string) (string, error) {
	code = strings.TrimSpace(code)
	if code == "" {
		return "", ResourceNotFonud
	}

	ctx, cancel := context.WithTimeout(context.Background(), avsoxLookupTimeout)
	defer cancel()

	movie, err := fetchAvsoxMovieByCode(ctx, code)
	if err != nil {
		return "", err
	}
	coverURL := firstNonEmpty(movie.PosterLarge, movie.PosterSmall)
	if coverURL == "" {
		return "", ResourceNotFonud
	}
	return coverURL, nil
}

// LookupSeriesURLByCode implements lookupProvider.
func (avsox) LookupSeriesURLByCode(code string) (string, error) {
	return "", errors.New("avsox: lookup series url not supported")
}

// LookupStudioURLByCode implements lookupProvider.
func (avsox) LookupStudioURLByCode(code string) (string, error) {
	return "", errors.New("avsox: lookup studio url not supported")
}

// LookupJavByCode fetches metadata for a given code.
func (avsox) LookupJavByCode(code string) (*JavInfo, error) {
	code = strings.TrimSpace(code)
	if code == "" {
		return nil, ResourceNotFonud
	}

	ctx, cancel := context.WithTimeout(context.Background(), avsoxLookupTimeout)
	defer cancel()

	movie, err := fetchAvsoxMovieByCode(ctx, code)
	if err != nil {
		return nil, err
	}

	info := avsoxMovieInfoFromAPI(movie)
	if info == nil {
		return nil, ResourceNotFonud
	}
	if info.Code == "" {
		info.Code = code
	}
	return info, nil
}

func fetchAvsoxMovieByCode(ctx context.Context, code string) (*avsoxAPIMovie, error) {
	session, err := cachedAvsoxSession(ctx, code)
	if err != nil {
		return nil, err
	}
	movie, err := fetchAvsoxMovieWithSession(ctx, session, code)
	if !isAvsoxSessionAuthError(err) {
		return movie, err
	}

	invalidateCachedAvsoxSession(session)
	logging.Info("avsox session expired, refreshing")
	session, err = refreshAvsoxSession(ctx, code)
	if err != nil {
		return nil, err
	}
	return fetchAvsoxMovieWithSession(ctx, session, code)
}

func fetchAvsoxMovieWithSession(ctx context.Context, session avsoxSession, code string) (*avsoxAPIMovie, error) {
	searchPayload := []any{
		map[string]string{
			"search": code,
			"lang":   avsoxAPILanguage,
		},
		avsoxAPISearchLimit,
		1,
	}
	var searchResults []avsoxAPIMovie
	if err := postAvsoxAPI(ctx, session, "/javu/data/api/search", searchPayload, &searchResults); err != nil {
		return nil, err
	}

	result := findAvsoxAPISearchResult(searchResults, code)
	if result == nil {
		return nil, ResourceNotFonud
	}
	if strings.TrimSpace(result.MovieID) == "" {
		return nil, ResourceNotFonud
	}
	detailPayload := []any{result.MovieID, avsoxAPILanguage}
	var movie avsoxAPIMovie
	if err := postAvsoxAPI(ctx, session, "/javu/data/api/getMovie", detailPayload, &movie); err != nil {
		return nil, err
	}
	if strings.TrimSpace(movie.MovieFanHao) == "" {
		movie.MovieFanHao = result.MovieFanHao
	}
	if strings.TrimSpace(movie.MovieID) == "" {
		movie.MovieID = result.MovieID
	}
	return &movie, nil
}

func cachedAvsoxSession(ctx context.Context, code string) (avsoxSession, error) {
	now := time.Now()
	avsoxSessionCache.Lock()
	session := avsoxSessionCache.session
	if session.csrfToken != "" && session.cookie != "" && now.Before(avsoxSessionCache.expiresAt) {
		avsoxSessionCache.Unlock()
		return session, nil
	}
	avsoxSessionCache.Unlock()
	return refreshAvsoxSession(ctx, code)
}

func refreshAvsoxSession(ctx context.Context, code string) (avsoxSession, error) {
	session, err := fetchAvsoxSession(ctx, code)
	if err != nil {
		return avsoxSession{}, err
	}
	avsoxSessionCache.Lock()
	avsoxSessionCache.session = session
	avsoxSessionCache.expiresAt = time.Now().Add(avsoxSessionTTL)
	avsoxSessionCache.Unlock()
	return session, nil
}

func invalidateCachedAvsoxSession(session avsoxSession) {
	avsoxSessionCache.Lock()
	if avsoxSessionCache.session.csrfToken == session.csrfToken && avsoxSessionCache.session.cookie == session.cookie {
		avsoxSessionCache.session = avsoxSession{}
		avsoxSessionCache.expiresAt = time.Time{}
	}
	avsoxSessionCache.Unlock()
}

func isAvsoxSessionAuthError(err error) bool {
	var statusErr avsoxStatusError
	if !errors.As(err, &statusErr) {
		return false
	}
	switch statusErr.status {
	case http.StatusBadRequest, http.StatusUnauthorized, http.StatusForbidden, 419:
		return true
	default:
		return false
	}
}

func findAvsoxAPISearchResult(results []avsoxAPIMovie, code string) *avsoxAPIMovie {
	wantCode := normalizeAvsoxCodeForCompare(code)
	for i := range results {
		result := &results[i]
		if normalizeAvsoxCodeForCompare(result.MovieFanHao) != wantCode {
			continue
		}
		if strings.TrimSpace(result.MovieID) == "" {
			continue
		}
		return result
	}
	return nil
}

func fetchAvsoxSession(ctx context.Context, code string) (avsoxSession, error) {
	pageURL := fmt.Sprintf("%s/%s/search/%s", avsoxBaseURL, avsoxAPILanguage, url.PathEscape(code))
	req, err := buildAvsoxRequest(ctx, pageURL, avsoxBaseURL)
	if err != nil {
		return avsoxSession{}, err
	}
	req.Header.Set("Sec-Fetch-Site", "none")
	req.Header.Set("Sec-Fetch-Mode", "navigate")
	req.Header.Set("Sec-Fetch-Dest", "document")

	logging.Info("avsox request: %s", pageURL)
	resp, err := doAvsoxRequest(req)
	if err != nil {
		return avsoxSession{}, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return avsoxSession{}, err
	}
	logging.Info("avsox response status: %s, length: %d bytes", resp.Status, len(body))

	if resp.StatusCode == http.StatusNotFound {
		return avsoxSession{}, ResourceNotFonud
	}
	if resp.StatusCode != http.StatusOK {
		return avsoxSession{}, fmt.Errorf("avsox: http %d", resp.StatusCode)
	}

	token := extractAvmooCSRFToken(string(body))
	cookie := avmooCookieHeader(resp.Cookies())
	if token == "" || cookie == "" {
		return avsoxSession{}, errors.New("avsox: missing csrf session")
	}
	return avsoxSession{
		csrfToken: token,
		cookie:    cookie,
		referer:   pageURL,
	}, nil
}

func postAvsoxAPI(ctx context.Context, session avsoxSession, path string, payload any, out any) error {
	var lastErr error
	for attempt := 1; attempt <= avsoxAPITries; attempt++ {
		err := postAvsoxAPIOnce(ctx, session, path, payload, out)
		if err == nil || errors.Is(err, ResourceNotFonud) {
			return err
		}
		lastErr = err
		if attempt == avsoxAPITries || !shouldRetryAvsoxAPIError(err) {
			break
		}
		logging.Info("avsox api retry after error: %v", err)
		timer := time.NewTimer(avsoxAPIRetryDelay)
		select {
		case <-ctx.Done():
			if !timer.Stop() {
				select {
				case <-timer.C:
				default:
				}
			}
			return ctx.Err()
		case <-timer.C:
		}
	}
	return lastErr
}

func shouldRetryAvsoxAPIError(err error) bool {
	if err == nil || errors.Is(err, ResourceNotFonud) {
		return false
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return true
	}
	var statusErr avsoxStatusError
	if errors.As(err, &statusErr) {
		return statusErr.status == http.StatusTooManyRequests || statusErr.status >= http.StatusInternalServerError
	}
	return false
}

func postAvsoxAPIOnce(ctx context.Context, session avsoxSession, path string, payload any, out any) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	targetURL := avsoxBaseURL + path
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, targetURL, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", avsoxUserAgent)
	req.Header.Set("Accept", "application/json, text/plain, */*")
	req.Header.Set("Accept-Language", "zh-CN,zh;q=0.9,en;q=0.8")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Origin", avsoxBaseURL)
	req.Header.Set("Referer", session.referer)
	req.Header.Set("X-Requested-With", "XMLHttpRequest")
	req.Header.Set("X-CSRF-Token", session.csrfToken)
	req.Header.Set("Cookie", session.cookie)

	logging.Info("avsox request: %s", targetURL)
	resp, err := doAvsoxRequest(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	logging.Info("avsox response status: %s, length: %d bytes", resp.Status, len(raw))

	if resp.StatusCode == http.StatusNotFound {
		return ResourceNotFonud
	}
	if resp.StatusCode != http.StatusOK {
		return avsoxStatusError{source: "http", status: resp.StatusCode}
	}

	var envelope avsoxAPIEnvelope
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return fmt.Errorf("avsox: parse api response: %w", err)
	}
	if envelope.Code == http.StatusNotFound {
		return ResourceNotFonud
	}
	if envelope.Code != http.StatusOK {
		return avsoxStatusError{source: "api", status: envelope.Code, message: envelope.Message}
	}
	if len(envelope.Data) == 0 || string(envelope.Data) == "null" {
		return ResourceNotFonud
	}
	if err := json.Unmarshal(envelope.Data, out); err != nil {
		return fmt.Errorf("avsox: parse api data: %w", err)
	}
	return nil
}

func avsoxMovieInfoFromAPI(movie *avsoxAPIMovie) *JavInfo {
	if movie == nil {
		return nil
	}
	isUncensored := true
	info := &JavInfo{
		Title:        firstNonEmpty(movie.Title, movie.TitleCN, movie.TitleTW, movie.TitleJA, movie.TitleEN),
		Code:         strings.TrimSpace(movie.MovieFanHao),
		ReleaseUnix:  parseDateUnix(movie.ReleaseDate),
		DurationMin:  movie.Length,
		CoverURL:     firstNonEmpty(movie.PosterLarge, movie.PosterSmall),
		IsUncensored: &isUncensored,
		Provider:     ProviderAvsox,
	}
	if movie.Studio != nil {
		info.Studio = firstNonEmpty(movie.Studio.StudioName, movie.Studio.StudioNameCN, movie.Studio.StudioNameTW, movie.Studio.StudioNameJA, movie.Studio.StudioNameEN)
	}
	if movie.Series != nil {
		info.Series = firstNonEmpty(movie.Series.SeriesName, movie.Series.SeriesNameCN, movie.Series.SeriesNameTW, movie.Series.SeriesNameJA, movie.Series.SeriesNameEN)
	}
	for _, genre := range movie.Genre {
		info.Tags = append(info.Tags, firstNonEmpty(genre.GenreName, genre.GenreNameCN, genre.GenreNameTW, genre.GenreNameJA, genre.GenreNameEN))
	}
	for _, star := range movie.Star {
		info.Actors = append(info.Actors, firstNonEmpty(star.StarName, star.StarNameCN, star.StarNameTW, star.StarNameJA, star.StarNameEN))
	}
	info.Tags = dedupeNonEmpty(info.Tags)
	info.Actors = dedupeNonEmpty(info.Actors)
	if info.Title == "" && info.Code == "" && info.Studio == "" && info.Series == "" && info.ReleaseUnix == 0 && info.DurationMin == 0 && len(info.Tags) == 0 && len(info.Actors) == 0 {
		return nil
	}
	return info
}

func fetchAvsoxDetailByCode(ctx context.Context, code string) (*html.Node, string, error) {
	searchURL := fmt.Sprintf("%s/cn/search/%s", avsoxBaseURL, url.PathEscape(code))
	searchDoc, status, err := fetchAvsoxHTML(ctx, searchURL, avsoxBaseURL)
	if err != nil {
		return nil, "", err
	}
	if status == http.StatusNotFound || searchDoc == nil {
		return nil, "", ResourceNotFonud
	}

	detailURL := findAvsoxSearchResultURL(searchDoc, code, searchURL)
	if detailURL == "" {
		return nil, "", ResourceNotFonud
	}

	detailDoc, status, err := fetchAvsoxHTML(ctx, detailURL, searchURL)
	if err != nil {
		return nil, "", err
	}
	if status == http.StatusNotFound || detailDoc == nil {
		return nil, "", ResourceNotFonud
	}
	return detailDoc, detailURL, nil
}

func fetchAvsoxHTML(ctx context.Context, targetURL, referer string) (*html.Node, int, error) {
	req, err := buildAvsoxRequest(ctx, targetURL, referer)
	if err != nil {
		return nil, 0, err
	}

	logging.Info("avsox request: %s", targetURL)
	resp, err := doAvsoxRequest(req)
	if err != nil {
		if errors.Is(err, util.ErrCachedNotFound) {
			return nil, http.StatusNotFound, nil
		}
		return nil, 0, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, resp.StatusCode, err
	}

	logging.Info("avsox response status: %s, length: %d bytes", resp.Status, len(body))
	if resp.StatusCode == http.StatusNotFound {
		return nil, resp.StatusCode, nil
	}
	if resp.StatusCode != http.StatusOK {
		return nil, resp.StatusCode, fmt.Errorf("avsox: http %d", resp.StatusCode)
	}

	doc, err := html.Parse(strings.NewReader(string(body)))
	if err != nil {
		return nil, resp.StatusCode, fmt.Errorf("avsox: parse html: %w", err)
	}
	return doc, resp.StatusCode, nil
}

func doAvsoxRequest(req *http.Request) (*http.Response, error) {
	if err := waitForAvsoxRateLimit(req.Context()); err != nil {
		return nil, err
	}
	return defaultAvsoxHTTPClient().Do(req)
}

func defaultAvsoxHTTPClient() *http.Client {
	avsoxHTTPClientOnce.Do(func() {
		avsoxHTTPClient = util.NewHTTPClientWithTransport(avsoxHTTPTimeout, func(t *http.Transport) {
			t.ForceAttemptHTTP2 = false
			t.DisableCompression = true
			t.TLSClientConfig = &tls.Config{MinVersion: tls.VersionTLS12, MaxVersion: tls.VersionTLS13}
			t.MaxIdleConns = 50
			t.MaxIdleConnsPerHost = 5
			t.MaxConnsPerHost = 5
		})
	})
	return avsoxHTTPClient
}

func waitForAvsoxRateLimit(ctx context.Context) error {
	for {
		avsoxRateLimiter.Lock()
		now := time.Now()
		if !now.Before(avsoxRateLimiter.next) {
			avsoxRateLimiter.next = now.Add(avsoxRequestInterval)
			avsoxRateLimiter.Unlock()
			return nil
		}
		wait := time.Until(avsoxRateLimiter.next)
		avsoxRateLimiter.Unlock()

		timer := time.NewTimer(wait)
		select {
		case <-ctx.Done():
			if !timer.Stop() {
				select {
				case <-timer.C:
				default:
				}
			}
			return fmt.Errorf("avsox: rate limit wait: %w", ctx.Err())
		case <-timer.C:
		}
	}
}

func buildAvsoxRequest(ctx context.Context, targetURL, referer string) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, targetURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", avsoxUserAgent)
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("Accept-Language", "zh-CN,zh;q=0.9,en;q=0.8")
	if referer != "" {
		req.Header.Set("Referer", referer)
	}
	return req, nil
}

func findAvsoxSearchResultURL(root *html.Node, code, pageURL string) string {
	waterfall := findElementByID(root, "waterfall")
	if waterfall == nil {
		return ""
	}

	wantCode := normalizeAvsoxCodeForCompare(code)
	for item := waterfall.FirstChild; item != nil; item = item.NextSibling {
		if item.Type != html.ElementNode || item.Data != "div" || !hasClass(item, "item") {
			continue
		}
		if normalizeAvsoxCodeForCompare(findAvsoxSearchItemCode(item)) != wantCode {
			continue
		}
		if href := firstAnchorHref(item); href != "" {
			return resolveURL(pageURL, href)
		}
	}
	return ""
}

func findAvsoxSearchItemCode(item *html.Node) string {
	var code string
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if code != "" {
			return
		}
		if n.Type == html.ElementNode && n.Data == "date" {
			text := strings.TrimSpace(flattenText(n))
			if text != "" && !isAvsoxReleaseDate(text) {
				code = text
				return
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(item)
	return code
}

type avsoxMovieFields struct {
	Title       string
	Code        string
	Studio      string
	Series      string
	ReleaseDate string
	Runtime     string
	Tags        []string
	Actors      []string
}

func parseAvsoxMovieInfo(root *html.Node) *JavInfo {
	scope := findAvsoxMainContainer(root)
	if scope == nil {
		return nil
	}

	fields := extractAvsoxMovieFields(scope)
	title := cleanAvsoxTitle(strings.TrimSpace(fields.Title), fields.Code)
	if title == "" {
		title = cleanAvsoxMoviePageTitle(strings.TrimSpace(firstTextByTag(root, "title")), fields.Code)
	}
	isUncensored := true

	info := &JavInfo{
		Title:        title,
		Code:         strings.TrimSpace(fields.Code),
		Studio:       strings.TrimSpace(fields.Studio),
		Series:       strings.TrimSpace(fields.Series),
		ReleaseUnix:  parseDateUnix(fields.ReleaseDate),
		DurationMin:  parseRuntimeMinutes(fields.Runtime),
		Tags:         dedupeNonEmpty(fields.Tags),
		Actors:       dedupeNonEmpty(fields.Actors),
		IsUncensored: &isUncensored,
		Provider:     ProviderAvsox,
	}
	if info.Title == "" && info.Code == "" && info.Studio == "" && info.Series == "" && info.ReleaseUnix == 0 && info.DurationMin == 0 && len(info.Tags) == 0 && len(info.Actors) == 0 {
		return nil
	}
	return info
}

func findAvsoxMainContainer(root *html.Node) *html.Node {
	var found *html.Node
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if found != nil {
			return
		}
		if n.Type == html.ElementNode && n.Data == "body" {
			for c := n.FirstChild; c != nil; c = c.NextSibling {
				if c.Type == html.ElementNode && c.Data == "div" && hasClass(c, "container") && findDescendantByClass(c, "div", "movie") != nil {
					found = c
					return
				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(root)
	return found
}

func extractAvsoxMovieFields(root *html.Node) avsoxMovieFields {
	var out avsoxMovieFields
	if root == nil {
		return out
	}

	out.Title = strings.TrimSpace(firstTextByTag(root, "h3"))
	if info := findDescendantByClass(root, "div", "info"); info != nil {
		out.Tags = collectAvmooGenreTexts(info)
		extractAvsoxInfoFields(info, &out)
	}
	if actors := findElementByID(root, "avatar-waterfall"); actors != nil {
		out.Actors = collectAnchorTexts(actors)
	}
	return out
}

func extractAvsoxInfoFields(root *html.Node, out *avsoxMovieFields) {
	pendingLabel := ""
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "p" {
			label, value := extractAvmooParagraphField(n)
			switch {
			case label != "" && value != "":
				pendingLabel = ""
				assignAvsoxMovieField(out, label, value)
			case label != "":
				pendingLabel = label
			case pendingLabel != "":
				assignAvsoxMovieField(out, pendingLabel, firstNonEmpty(firstAnchorText(n), flattenText(n)))
				pendingLabel = ""
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(root)
}

func assignAvsoxMovieField(out *avsoxMovieFields, label, value string) {
	if out == nil {
		return
	}
	label = normalizeAvmooLabel(label)
	value = strings.TrimSpace(value)
	if label == "" || value == "" {
		return
	}

	switch label {
	case "識別碼", "识别码", "番號", "番号":
		if out.Code == "" {
			out.Code = value
		}
	case "發行日期", "发行日期", "發行時間", "发行时间", "発売日", "releasedate":
		if out.ReleaseDate == "" {
			out.ReleaseDate = value
		}
	case "長度", "长度", "時長", "时长", "duration", "runtime":
		if out.Runtime == "" {
			out.Runtime = value
		}
	case "製作商", "制作商", "studio", "メーカー", "メーカー名":
		if out.Studio == "" {
			out.Studio = value
		}
	case "系列", "series":
		if out.Series == "" {
			out.Series = value
		}
	}
}

func parseAvsoxCoverURL(root *html.Node, pageURL string) string {
	return parseAvmooCoverURL(root, pageURL)
}

func cleanAvsoxMoviePageTitle(title, code string) string {
	title = strings.TrimSpace(title)
	if title == "" {
		return ""
	}
	title = strings.TrimSuffix(title, "- AVSOX")
	return cleanAvsoxTitle(title, code)
}

func cleanAvsoxTitle(title, code string) string {
	title = strings.TrimSpace(title)
	code = strings.TrimSpace(code)
	if title == "" || code == "" {
		return title
	}
	if len(title) >= len(code) && strings.EqualFold(title[:len(code)], code) {
		return strings.TrimSpace(title[len(code):])
	}
	return title
}

func normalizeAvsoxCodeForCompare(code string) string {
	return strings.ToUpper(strings.TrimSpace(code))
}

func isAvsoxReleaseDate(value string) bool {
	value = strings.TrimSpace(value)
	return len(value) == len("2006-01-02") && parseDateUnix(value) != 0
}
