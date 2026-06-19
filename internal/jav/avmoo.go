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
	"regexp"
	"strings"
	"sync"
	"time"

	"golang.org/x/net/html"

	"pornboss/internal/common/logging"
	"pornboss/internal/util"
)

// avmoo implements lookupProvider.
type avmoo struct{}

var avmooProvider lookupProvider = avmoo{}

const (
	avmooBaseURL         = "https://avmoo.shop"
	avmooUserAgent       = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"
	avmooRequestInterval = 1500 * time.Millisecond
	avmooAPILanguage     = "tw"
	avmooAPISearchLimit  = 30
	avmooLookupTimeout   = 90 * time.Second
	avmooHTTPTimeout     = 30 * time.Second
	avmooAPITries        = 3
	avmooAPIRetryDelay   = 2 * time.Second
	avmooSessionTTL      = 30 * time.Minute
)

var avmooRateLimiter = struct {
	sync.Mutex
	next time.Time
}{}

var (
	avmooHTTPClientOnce sync.Once
	avmooHTTPClient     *http.Client
)

var avmooSessionCache = struct {
	sync.Mutex
	session   avmooSession
	expiresAt time.Time
}{}

type avmooSession struct {
	csrfToken string
	cookie    string
	referer   string
}

type avmooAPIEnvelope struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data"`
}

type avmooStatusError struct {
	source  string
	status  int
	message string
}

func (e avmooStatusError) Error() string {
	if strings.TrimSpace(e.message) != "" {
		return fmt.Sprintf("avmoo: %s code %d: %s", e.source, e.status, e.message)
	}
	return fmt.Sprintf("avmoo: %s code %d", e.source, e.status)
}

type avmooAPIMovie struct {
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
	Studio      *avmooAPIStudio `json:"studio"`
	Series      *avmooAPISeries `json:"series"`
	Genre       []avmooAPIGenre `json:"genre"`
	Star        []avmooAPIStar  `json:"star"`
}

type avmooAPIStudio struct {
	StudioName   string `json:"studioName"`
	StudioNameJA string `json:"studioName_ja"`
	StudioNameEN string `json:"studioName_en"`
	StudioNameCN string `json:"studioName_cn"`
	StudioNameTW string `json:"studioName_tw"`
}

type avmooAPISeries struct {
	SeriesName   string `json:"seriesName"`
	SeriesNameJA string `json:"seriesName_ja"`
	SeriesNameEN string `json:"seriesName_en"`
	SeriesNameCN string `json:"seriesName_cn"`
	SeriesNameTW string `json:"seriesName_tw"`
}

type avmooAPIGenre struct {
	GenreName   string `json:"genreName"`
	GenreNameJA string `json:"genreName_ja"`
	GenreNameEN string `json:"genreName_en"`
	GenreNameCN string `json:"genreName_cn"`
	GenreNameTW string `json:"genreName_tw"`
}

type avmooAPIStar struct {
	StarName   string `json:"starName"`
	StarNameJA string `json:"starName_ja"`
	StarNameEN string `json:"starName_en"`
	StarNameCN string `json:"starName_cn"`
	StarNameTW string `json:"starName_tw"`
}

// LookupActressByName implements lookupProvider.
func (avmoo) LookupActressByName(name string) (*ActressInfo, error) {
	return nil, errors.New("avmoo: lookup actress not supported")
}

// LookupActressByCode implements lookupProvider.
func (avmoo) LookupActressByCode(code string) (*ActressInfo, error) {
	return nil, errors.New("avmoo: lookup actress not supported")
}

// LookupActressURLByCodeAndName implements lookupProvider.
func (avmoo) LookupActressURLByCodeAndName(code, name string) (string, error) {
	return "", errors.New("avmoo: lookup actress url not supported")
}

// LookupCoverURLByCode resolves a cover image URL for a movie code.
func (avmoo) LookupCoverURLByCode(code string) (string, error) {
	code = strings.TrimSpace(code)
	if code == "" {
		return "", ResourceNotFonud
	}

	ctx, cancel := context.WithTimeout(context.Background(), avmooLookupTimeout)
	defer cancel()

	movie, err := fetchAvmooMovieByCode(ctx, code)
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
func (avmoo) LookupSeriesURLByCode(code string) (string, error) {
	return "", errors.New("avmoo: lookup series url not supported")
}

// LookupStudioURLByCode implements lookupProvider.
func (avmoo) LookupStudioURLByCode(code string) (string, error) {
	return "", errors.New("avmoo: lookup studio url not supported")
}

// LookupJavByCode fetches metadata for a given code.
func (avmoo) LookupJavByCode(code string) (*JavInfo, error) {
	code = strings.TrimSpace(code)
	if code == "" {
		return nil, ResourceNotFonud
	}

	ctx, cancel := context.WithTimeout(context.Background(), avmooLookupTimeout)
	defer cancel()

	movie, err := fetchAvmooMovieByCode(ctx, code)
	if err != nil {
		return nil, err
	}

	info := avmooMovieInfoFromAPI(movie)
	if info == nil {
		return nil, ResourceNotFonud
	}
	if info.Code == "" {
		info.Code = code
	}
	return info, nil
}

func fetchAvmooMovieByCode(ctx context.Context, code string) (*avmooAPIMovie, error) {
	session, err := cachedAvmooSession(ctx, code)
	if err != nil {
		return nil, err
	}
	movie, err := fetchAvmooMovieWithSession(ctx, session, code)
	if !isAvmooSessionAuthError(err) {
		return movie, err
	}

	invalidateCachedAvmooSession(session)
	logging.Info("avmoo session expired, refreshing")
	session, err = refreshAvmooSession(ctx, code)
	if err != nil {
		return nil, err
	}
	return fetchAvmooMovieWithSession(ctx, session, code)
}

func fetchAvmooMovieWithSession(ctx context.Context, session avmooSession, code string) (*avmooAPIMovie, error) {
	searchPayload := []any{
		map[string]string{
			"search": code,
			"lang":   avmooAPILanguage,
		},
		avmooAPISearchLimit,
		1,
	}
	var searchResults []avmooAPIMovie
	if err := postAvmooAPI(ctx, session, "/jav/data/api/search", searchPayload, &searchResults); err != nil {
		return nil, err
	}

	result := findAvmooAPISearchResult(searchResults, code)
	if result == nil {
		return nil, ResourceNotFonud
	}
	if strings.TrimSpace(result.MovieID) == "" {
		return nil, ResourceNotFonud
	}
	detailPayload := []any{result.MovieID, avmooAPILanguage}
	var movie avmooAPIMovie
	if err := postAvmooAPI(ctx, session, "/jav/data/api/getMovie", detailPayload, &movie); err != nil {
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

func cachedAvmooSession(ctx context.Context, code string) (avmooSession, error) {
	now := time.Now()
	avmooSessionCache.Lock()
	session := avmooSessionCache.session
	if session.csrfToken != "" && session.cookie != "" && now.Before(avmooSessionCache.expiresAt) {
		avmooSessionCache.Unlock()
		return session, nil
	}
	avmooSessionCache.Unlock()
	return refreshAvmooSession(ctx, code)
}

func refreshAvmooSession(ctx context.Context, code string) (avmooSession, error) {
	session, err := fetchAvmooSession(ctx, code)
	if err != nil {
		return avmooSession{}, err
	}
	avmooSessionCache.Lock()
	avmooSessionCache.session = session
	avmooSessionCache.expiresAt = time.Now().Add(avmooSessionTTL)
	avmooSessionCache.Unlock()
	return session, nil
}

func invalidateCachedAvmooSession(session avmooSession) {
	avmooSessionCache.Lock()
	if avmooSessionCache.session.csrfToken == session.csrfToken && avmooSessionCache.session.cookie == session.cookie {
		avmooSessionCache.session = avmooSession{}
		avmooSessionCache.expiresAt = time.Time{}
	}
	avmooSessionCache.Unlock()
}

func isAvmooSessionAuthError(err error) bool {
	var statusErr avmooStatusError
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

func findAvmooAPISearchResult(results []avmooAPIMovie, code string) *avmooAPIMovie {
	wantCode := normalizeAvmooCode(code)
	for i := range results {
		result := &results[i]
		if normalizeAvmooCode(result.MovieFanHao) != wantCode {
			continue
		}
		if strings.TrimSpace(result.MovieID) == "" {
			continue
		}
		return result
	}
	return nil
}

func fetchAvmooSession(ctx context.Context, code string) (avmooSession, error) {
	pageURL := fmt.Sprintf("%s/%s/search/%s", avmooBaseURL, avmooAPILanguage, url.PathEscape(code))
	req, err := buildAvmooRequest(ctx, pageURL, avmooBaseURL)
	if err != nil {
		return avmooSession{}, err
	}
	req.Header.Set("Sec-Fetch-Site", "none")
	req.Header.Set("Sec-Fetch-Mode", "navigate")
	req.Header.Set("Sec-Fetch-Dest", "document")

	logging.Info("avmoo request: %s", pageURL)
	resp, err := doAvmooRequest(req)
	if err != nil {
		return avmooSession{}, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return avmooSession{}, err
	}
	logging.Info("avmoo response status: %s, length: %d bytes", resp.Status, len(body))

	if resp.StatusCode == http.StatusNotFound {
		return avmooSession{}, ResourceNotFonud
	}
	if resp.StatusCode != http.StatusOK {
		return avmooSession{}, fmt.Errorf("avmoo: http %d", resp.StatusCode)
	}

	token := extractAvmooCSRFToken(string(body))
	cookie := avmooCookieHeader(resp.Cookies())
	if token == "" || cookie == "" {
		return avmooSession{}, errors.New("avmoo: missing csrf session")
	}
	return avmooSession{
		csrfToken: token,
		cookie:    cookie,
		referer:   pageURL,
	}, nil
}

func postAvmooAPI(ctx context.Context, session avmooSession, path string, payload any, out any) error {
	var lastErr error
	for attempt := 1; attempt <= avmooAPITries; attempt++ {
		err := postAvmooAPIOnce(ctx, session, path, payload, out)
		if err == nil || errors.Is(err, ResourceNotFonud) {
			return err
		}
		lastErr = err
		if attempt == avmooAPITries || !shouldRetryAvmooAPIError(err) {
			break
		}
		logging.Info("avmoo api retry after error: %v", err)
		timer := time.NewTimer(avmooAPIRetryDelay)
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

func shouldRetryAvmooAPIError(err error) bool {
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
	var statusErr avmooStatusError
	if errors.As(err, &statusErr) {
		return statusErr.status == http.StatusTooManyRequests || statusErr.status >= http.StatusInternalServerError
	}
	return false
}

func postAvmooAPIOnce(ctx context.Context, session avmooSession, path string, payload any, out any) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	targetURL := avmooBaseURL + path
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, targetURL, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", avmooUserAgent)
	req.Header.Set("Accept", "application/json, text/plain, */*")
	req.Header.Set("Accept-Language", "zh-TW,zh;q=0.9,en;q=0.8")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Origin", avmooBaseURL)
	req.Header.Set("Referer", session.referer)
	req.Header.Set("X-Requested-With", "XMLHttpRequest")
	req.Header.Set("X-CSRF-Token", session.csrfToken)
	req.Header.Set("Cookie", session.cookie)

	logging.Info("avmoo request: %s", targetURL)
	resp, err := doAvmooRequest(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	logging.Info("avmoo response status: %s, length: %d bytes", resp.Status, len(raw))

	if resp.StatusCode == http.StatusNotFound {
		return ResourceNotFonud
	}
	if resp.StatusCode != http.StatusOK {
		return avmooStatusError{source: "http", status: resp.StatusCode}
	}

	var envelope avmooAPIEnvelope
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return fmt.Errorf("avmoo: parse api response: %w", err)
	}
	if envelope.Code == http.StatusNotFound {
		return ResourceNotFonud
	}
	if envelope.Code != http.StatusOK {
		return avmooStatusError{source: "api", status: envelope.Code, message: envelope.Message}
	}
	if len(envelope.Data) == 0 || string(envelope.Data) == "null" {
		return ResourceNotFonud
	}
	if err := json.Unmarshal(envelope.Data, out); err != nil {
		return fmt.Errorf("avmoo: parse api data: %w", err)
	}
	return nil
}

func avmooMovieInfoFromAPI(movie *avmooAPIMovie) *JavInfo {
	if movie == nil {
		return nil
	}
	info := &JavInfo{
		Title:       firstNonEmpty(movie.Title, movie.TitleTW, movie.TitleCN, movie.TitleJA, movie.TitleEN),
		Code:        strings.TrimSpace(movie.MovieFanHao),
		ReleaseUnix: parseDateUnix(movie.ReleaseDate),
		DurationMin: movie.Length,
		CoverURL:    firstNonEmpty(movie.PosterLarge, movie.PosterSmall),
		Provider:    ProviderAvmoo,
	}
	if movie.Studio != nil {
		info.Studio = firstNonEmpty(movie.Studio.StudioName, movie.Studio.StudioNameTW, movie.Studio.StudioNameCN, movie.Studio.StudioNameJA, movie.Studio.StudioNameEN)
	}
	if movie.Series != nil {
		info.Series = firstNonEmpty(movie.Series.SeriesName, movie.Series.SeriesNameTW, movie.Series.SeriesNameCN, movie.Series.SeriesNameJA, movie.Series.SeriesNameEN)
	}
	for _, genre := range movie.Genre {
		info.Tags = append(info.Tags, firstNonEmpty(genre.GenreName, genre.GenreNameTW, genre.GenreNameCN, genre.GenreNameJA, genre.GenreNameEN))
	}
	for _, star := range movie.Star {
		info.Actors = append(info.Actors, firstNonEmpty(star.StarName, star.StarNameTW, star.StarNameCN, star.StarNameJA, star.StarNameEN))
	}
	info.Tags = dedupeNonEmpty(info.Tags)
	info.Actors = dedupeNonEmpty(info.Actors)
	if info.Title == "" && info.Code == "" && info.Studio == "" && info.Series == "" && info.ReleaseUnix == 0 && info.DurationMin == 0 && len(info.Tags) == 0 && len(info.Actors) == 0 {
		return nil
	}
	return info
}

func extractAvmooCSRFToken(body string) string {
	re := regexp.MustCompile(`<meta\s+name=["']csrf-token["']\s+content=["']([^"']+)["']`)
	match := re.FindStringSubmatch(body)
	if len(match) < 2 {
		return ""
	}
	return strings.TrimSpace(match[1])
}

func avmooCookieHeader(cookies []*http.Cookie) string {
	var parts []string
	for _, cookie := range cookies {
		if cookie == nil || strings.TrimSpace(cookie.Name) == "" {
			continue
		}
		parts = append(parts, cookie.Name+"="+cookie.Value)
	}
	return strings.Join(parts, "; ")
}

func fetchAvmooDetailByCode(ctx context.Context, code string) (*html.Node, string, error) {
	searchURL := fmt.Sprintf("%s/tw/search/%s", avmooBaseURL, url.PathEscape(code))
	searchDoc, status, err := fetchAvmooHTML(ctx, searchURL, avmooBaseURL)
	if err != nil {
		return nil, "", err
	}
	if status == http.StatusNotFound || searchDoc == nil {
		return nil, "", ResourceNotFonud
	}

	detailURL := findAvmooSearchResultURL(searchDoc, code, searchURL)
	if detailURL == "" {
		return nil, "", ResourceNotFonud
	}

	detailDoc, status, err := fetchAvmooHTML(ctx, detailURL, searchURL)
	if err != nil {
		return nil, "", err
	}
	if status == http.StatusNotFound || detailDoc == nil {
		return nil, "", ResourceNotFonud
	}
	return detailDoc, detailURL, nil
}

func fetchAvmooHTML(ctx context.Context, targetURL, referer string) (*html.Node, int, error) {
	req, err := buildAvmooRequest(ctx, targetURL, referer)
	if err != nil {
		return nil, 0, err
	}

	logging.Info("avmoo request: %s", targetURL)
	resp, err := doAvmooRequest(req)
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

	logging.Info("avmoo response status: %s, length: %d bytes", resp.Status, len(body))
	if resp.StatusCode == http.StatusNotFound {
		return nil, resp.StatusCode, nil
	}
	if resp.StatusCode != http.StatusOK {
		return nil, resp.StatusCode, fmt.Errorf("avmoo: http %d", resp.StatusCode)
	}

	doc, err := html.Parse(strings.NewReader(string(body)))
	if err != nil {
		return nil, resp.StatusCode, fmt.Errorf("avmoo: parse html: %w", err)
	}
	return doc, resp.StatusCode, nil
}

func doAvmooRequest(req *http.Request) (*http.Response, error) {
	if err := waitForAvmooRateLimit(req.Context()); err != nil {
		return nil, err
	}
	return defaultAvmooHTTPClient().Do(req)
}

func defaultAvmooHTTPClient() *http.Client {
	avmooHTTPClientOnce.Do(func() {
		avmooHTTPClient = util.NewHTTPClientWithTransport(avmooHTTPTimeout, func(t *http.Transport) {
			t.ForceAttemptHTTP2 = false
			t.DisableCompression = true
			t.TLSClientConfig = &tls.Config{MinVersion: tls.VersionTLS12, MaxVersion: tls.VersionTLS13}
			t.MaxIdleConns = 50
			t.MaxIdleConnsPerHost = 5
			t.MaxConnsPerHost = 5
		})
	})
	return avmooHTTPClient
}

func waitForAvmooRateLimit(ctx context.Context) error {
	for {
		avmooRateLimiter.Lock()
		now := time.Now()
		if !now.Before(avmooRateLimiter.next) {
			avmooRateLimiter.next = now.Add(avmooRequestInterval)
			avmooRateLimiter.Unlock()
			return nil
		}
		wait := time.Until(avmooRateLimiter.next)
		avmooRateLimiter.Unlock()

		timer := time.NewTimer(wait)
		select {
		case <-ctx.Done():
			if !timer.Stop() {
				select {
				case <-timer.C:
				default:
				}
			}
			return fmt.Errorf("avmoo: rate limit wait: %w", ctx.Err())
		case <-timer.C:
		}
	}
}

func buildAvmooRequest(ctx context.Context, targetURL, referer string) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, targetURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", avmooUserAgent)
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("Accept-Language", "zh-TW,zh;q=0.9,en;q=0.8")
	if referer != "" {
		req.Header.Set("Referer", referer)
	}
	return req, nil
}

func findAvmooSearchResultURL(root *html.Node, code, pageURL string) string {
	waterfall := findElementByID(root, "waterfall")
	if waterfall == nil {
		return ""
	}

	wantCode := normalizeAvmooCode(code)
	for item := waterfall.FirstChild; item != nil; item = item.NextSibling {
		if item.Type != html.ElementNode || item.Data != "div" || !hasClass(item, "item") {
			continue
		}
		if normalizeAvmooCode(findAvmooSearchItemCode(item)) != wantCode {
			continue
		}
		if href := firstAnchorHref(item); href != "" {
			return resolveURL(pageURL, href)
		}
	}
	return ""
}

func findAvmooSearchItemCode(item *html.Node) string {
	var code string
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if code != "" {
			return
		}
		if n.Type == html.ElementNode && n.Data == "date" {
			text := strings.TrimSpace(flattenText(n))
			if util.CodeRe.MatchString(text) {
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

type avmooMovieFields struct {
	Title       string
	Code        string
	Series      string
	ReleaseDate string
	Runtime     string
	Tags        []string
	Actors      []string
}

func parseAvmooMovieInfo(root *html.Node) *JavInfo {
	scope := findAvmooMainContainer(root)
	if scope == nil {
		return nil
	}

	fields := extractAvmooMovieFields(scope)
	title := strings.TrimSpace(fields.Title)
	if title == "" {
		title = cleanAvmooMoviePageTitle(strings.TrimSpace(firstTextByTag(root, "title")))
	}

	info := &JavInfo{
		Title:       title,
		Code:        strings.TrimSpace(fields.Code),
		Series:      strings.TrimSpace(fields.Series),
		ReleaseUnix: parseDateUnix(fields.ReleaseDate),
		DurationMin: parseRuntimeMinutes(fields.Runtime),
		Tags:        dedupeNonEmpty(fields.Tags),
		Actors:      dedupeNonEmpty(fields.Actors),
		CoverURL:    parseAvmooCoverURL(root, ""),
		Provider:    ProviderAvmoo,
	}
	if info.Title == "" && info.Code == "" && info.Series == "" && info.ReleaseUnix == 0 && info.DurationMin == 0 && len(info.Tags) == 0 && len(info.Actors) == 0 {
		return nil
	}
	return info
}

func findAvmooMainContainer(root *html.Node) *html.Node {
	var found *html.Node
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if found != nil {
			return
		}
		if n.Type == html.ElementNode && n.Data == "div" && hasClass(n, "container") {
			if findDescendantByClass(n, "div", "movie") != nil {
				found = n
				return
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(root)
	return found
}

func extractAvmooMovieFields(root *html.Node) avmooMovieFields {
	var out avmooMovieFields
	if root == nil {
		return out
	}

	out.Title = cleanAvmooTitle(strings.TrimSpace(firstTextByTag(root, "h3")))
	if info := findDescendantByClass(root, "div", "info"); info != nil {
		out.Tags = collectAvmooGenreTexts(info)
		extractAvmooInfoFields(info, &out)
	}
	if actors := findElementByID(root, "avatar-waterfall"); actors != nil {
		out.Actors = collectAnchorTexts(actors)
	}
	return out
}

func extractAvmooInfoFields(root *html.Node, out *avmooMovieFields) {
	pendingLabel := ""
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "p" {
			label, value := extractAvmooParagraphField(n)
			switch {
			case label != "" && value != "":
				pendingLabel = ""
				assignAvmooMovieField(out, label, value)
			case label != "":
				pendingLabel = label
			case pendingLabel != "":
				assignAvmooMovieField(out, pendingLabel, firstNonEmpty(firstAnchorText(n), flattenText(n)))
				pendingLabel = ""
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(root)
}

func extractAvmooParagraphField(p *html.Node) (string, string) {
	if hasClass(p, "header") {
		return strings.TrimSpace(flattenText(p)), ""
	}
	for c := p.FirstChild; c != nil; c = c.NextSibling {
		if c.Type == html.ElementNode && c.Data == "span" && hasClass(c, "header") {
			label := strings.TrimSpace(flattenText(c))
			value := collectTextAfterNode(c)
			return label, value
		}
	}
	return "", ""
}

func assignAvmooMovieField(out *avmooMovieFields, label, value string) {
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
	case "發行日期", "发行日期", "発売日", "release date":
		if out.ReleaseDate == "" {
			out.ReleaseDate = value
		}
	case "長度", "长度", "時長", "时长", "duration", "runtime":
		if out.Runtime == "" {
			out.Runtime = value
		}
	case "系列", "series":
		if out.Series == "" {
			out.Series = value
		}
	}
}

func collectAvmooGenreTexts(root *html.Node) []string {
	if root == nil {
		return nil
	}

	seen := make(map[string]struct{})
	var texts []string
	var walk func(*html.Node, bool)
	walk = func(n *html.Node, inGenre bool) {
		in := inGenre || (n.Type == html.ElementNode && n.Data == "span" && hasClass(n, "genre"))
		if n.Type == html.ElementNode && n.Data == "a" && in {
			text := strings.TrimSpace(flattenText(n))
			if text != "" {
				if _, ok := seen[text]; !ok {
					seen[text] = struct{}{}
					texts = append(texts, text)
				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c, in)
		}
	}
	walk(root, false)
	return texts
}

func parseAvmooCoverURL(root *html.Node, pageURL string) string {
	if root == nil {
		return ""
	}

	var cover string
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if cover != "" {
			return
		}
		if n.Type == html.ElementNode && n.Data == "a" && hasClass(n, "bigImage") {
			if href := strings.TrimSpace(attrValue(n, "href")); href != "" {
				cover = resolveURL(pageURL, href)
				return
			}
		}
		if n.Type == html.ElementNode && n.Data == "img" {
			if src := strings.TrimSpace(attrValue(n, "src")); src != "" && isInsideClass(n, "screencap") {
				cover = resolveURL(pageURL, src)
				return
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(root)
	return strings.TrimSpace(cover)
}

func findElementByID(root *html.Node, id string) *html.Node {
	var found *html.Node
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if found != nil {
			return
		}
		if n.Type == html.ElementNode && attrValue(n, "id") == id {
			found = n
			return
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(root)
	return found
}

func findDescendantByClass(root *html.Node, tag, class string) *html.Node {
	var found *html.Node
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if found != nil {
			return
		}
		if n.Type == html.ElementNode && n.Data == tag && hasClass(n, class) {
			found = n
			return
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(root)
	return found
}

func isInsideClass(n *html.Node, class string) bool {
	for p := n.Parent; p != nil; p = p.Parent {
		if p.Type == html.ElementNode && hasClass(p, class) {
			return true
		}
	}
	return false
}

func collectTextAfterNode(node *html.Node) string {
	var b strings.Builder
	for cur := node.NextSibling; cur != nil; cur = cur.NextSibling {
		if cur.Type == html.ElementNode {
			text := strings.TrimSpace(flattenText(cur))
			if text != "" {
				if b.Len() > 0 {
					b.WriteString(" ")
				}
				b.WriteString(text)
			}
			continue
		}
		if cur.Type == html.TextNode {
			text := strings.TrimSpace(cur.Data)
			if text != "" {
				if b.Len() > 0 {
					b.WriteString(" ")
				}
				b.WriteString(text)
			}
		}
	}
	return strings.TrimSpace(b.String())
}

func cleanAvmooMoviePageTitle(title string) string {
	title = strings.TrimSpace(title)
	if title == "" {
		return ""
	}
	title = strings.TrimSuffix(title, "- AVMOO")
	return cleanAvmooTitle(title)
}

func cleanAvmooTitle(title string) string {
	title = strings.TrimSpace(title)
	if title == "" {
		return ""
	}
	re := regexp.MustCompile(`(?i)^[a-z]{2,6}[-_ ]?\d{2,5}[a-z]{0,2}\s+`)
	title = re.ReplaceAllString(title, "")
	return strings.TrimSpace(title)
}

func normalizeAvmooLabel(label string) string {
	label = strings.ToLower(strings.TrimSpace(label))
	label = strings.TrimSuffix(label, ":")
	label = strings.TrimSuffix(label, "：")
	return strings.Join(strings.Fields(label), "")
}

func normalizeAvmooCode(code string) string {
	code = strings.ToUpper(strings.TrimSpace(code))
	var b strings.Builder
	for _, r := range code {
		if (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
		}
	}
	return b.String()
}
