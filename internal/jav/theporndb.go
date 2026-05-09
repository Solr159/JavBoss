package jav

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"pornboss/internal/common/logging"
	"pornboss/internal/util"
)

// thePornDB implements lookupProvider.
type thePornDB struct{}

var thePornDBProvider lookupProvider = thePornDB{}

const thePornDBBearerToken = "uqtWi1LRXC2ngClxz8QrqfOERuH2qbuh89CQAiXx85088612"

// LookupActressByCode implements lookupProvider.
func (thePornDB) LookupActressByCode(code string) (*ActressInfo, error) {
	return nil, errors.New("theporndb: lookup actress not supported")
}

// LookupActressByJapaneseName implements lookupProvider.
func (thePornDB) LookupActressByJapaneseName(name string) (*ActressInfo, error) {
	return nil, errors.New("theporndb: lookup actress not supported")
}

// LookupJavByCode implements lookupProvider.
func (thePornDB) LookupJavByCode(code string) (*JavInfo, error) {
	code = strings.TrimSpace(code)
	if code == "" {
		return nil, ResourceNotFonud
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	payload, err := fetchThePornDBJavByCode(ctx, code)
	if err != nil {
		return nil, err
	}
	record := findThePornDBRecordByCode(payload, code)
	if record == nil {
		return nil, ResourceNotFonud
	}
	info := parseThePornDBJavInfo(*record)
	if info == nil {
		return nil, ResourceNotFonud
	}
	if info.Code == "" {
		info.Code = normalizeThePornDBCodeDisplay(code)
	}
	return info, nil
}

// LookupCoverURLByCode implements lookupProvider.
func (thePornDB) LookupCoverURLByCode(code string) (string, error) {
	code = strings.ToLower(strings.TrimSpace(code))
	if code == "" {
		return "", ResourceNotFonud
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	payload, err := fetchThePornDBJavByCode(ctx, code)
	if err != nil {
		return "", err
	}
	record := findThePornDBRecordByCode(payload, code)
	if record == nil {
		return "", ResourceNotFonud
	}
	coverURL := strings.TrimSpace(record.Background.Full)
	if coverURL == "" {
		return "", ResourceNotFonud
	}
	return coverURL, nil
}

func fetchThePornDBJavByCode(ctx context.Context, code string) (*thePornDBResponse, error) {
	code = strings.ToLower(strings.TrimSpace(code))
	if code == "" {
		return nil, ResourceNotFonud
	}

	targetURL := fmt.Sprintf("https://api.theporndb.net/jav?external_id=%s", url.QueryEscape(code))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, targetURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+thePornDBBearerToken)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; PornBoss/1.0)")

	logging.Info("theporndb request: %s", targetURL)
	resp, err := util.DoRequest(req)
	if err != nil {
		if errors.Is(err, util.ErrCachedNotFound) {
			return nil, ResourceNotFonud
		}
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, ResourceNotFonud
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("theporndb: http %d", resp.StatusCode)
	}

	payload, err := decodeThePornDBResponse(io.LimitReader(resp.Body, 2*1024*1024))
	if err != nil {
		return nil, fmt.Errorf("theporndb: decode response: %w", err)
	}
	if payload == nil || len(payload.Data) == 0 {
		return nil, ResourceNotFonud
	}
	return payload, nil
}

func findThePornDBRecordByCode(payload *thePornDBResponse, code string) *thePornDBRecord {
	if payload == nil {
		return nil
	}
	wantCode := normalizeThePornDBCode(code)
	for i := range payload.Data {
		if normalizeThePornDBCode(payload.Data[i].ExternalID) != wantCode {
			continue
		}
		return &payload.Data[i]
	}
	return nil
}

func parseThePornDBJavInfo(item thePornDBRecord) *JavInfo {
	info := &JavInfo{
		Title:       cleanThePornDBTitle(item.Title, item.ExternalID),
		Code:        normalizeThePornDBCodeDisplay(item.ExternalID),
		ReleaseUnix: parseDateUnix(item.Date),
		DurationMin: parseThePornDBDurationMinutes(item.Duration),
		Tags:        dedupeNonEmpty(collectThePornDBTagNames(item.Tags)),
		Actors:      dedupeNonEmpty(collectThePornDBPerformerNames(item.Performers)),
		Provider:    ProviderThePornDB,
	}
	if info.Title == "" && info.Code == "" && info.ReleaseUnix == 0 && info.DurationMin == 0 && len(info.Tags) == 0 && len(info.Actors) == 0 {
		return nil
	}
	return info
}

type thePornDBResponse struct {
	Data []thePornDBRecord `json:"data"`
}

type thePornDBRecord struct {
	ExternalID string `json:"external_id"`
	Title      string `json:"title"`
	Date       string `json:"date"`
	Duration   int    `json:"duration"`
	Background struct {
		Full   string `json:"full"`
		Large  string `json:"large"`
		Medium string `json:"medium"`
		Small  string `json:"small"`
	} `json:"background"`
	Performers []thePornDBPerformer `json:"performers"`
	Tags       []thePornDBTag       `json:"tags"`
}

type thePornDBPerformer struct {
	Name   string `json:"name"`
	Parent struct {
		Name     string `json:"name"`
		FullName string `json:"full_name"`
	} `json:"parent"`
}

type thePornDBTag struct {
	Name string `json:"name"`
}

func decodeThePornDBResponse(body io.Reader) (*thePornDBResponse, error) {
	var payload thePornDBResponse
	if err := json.NewDecoder(body).Decode(&payload); err != nil {
		return nil, err
	}
	return &payload, nil
}

func collectThePornDBPerformerNames(values []thePornDBPerformer) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		name := strings.TrimSpace(value.Parent.FullName)
		if name == "" {
			name = strings.TrimSpace(value.Parent.Name)
		}
		if name == "" {
			name = strings.TrimSpace(value.Name)
		}
		if name != "" {
			out = append(out, name)
		}
	}
	return out
}

func collectThePornDBTagNames(values []thePornDBTag) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		if name := strings.TrimSpace(value.Name); name != "" {
			out = append(out, name)
		}
	}
	return out
}

func parseThePornDBDurationMinutes(value int) int {
	if value <= 0 {
		return 0
	}
	return value / 60
}

func cleanThePornDBTitle(title, code string) string {
	title = strings.TrimSpace(title)
	if title == "" {
		return ""
	}
	codePattern := regexp.QuoteMeta(strings.TrimSpace(code))
	if codePattern != "" {
		re := regexp.MustCompile(`(?i)^` + codePattern + `\s*[:：-]\s*`)
		title = re.ReplaceAllString(title, "")
	}
	re := regexp.MustCompile(`(?i)^[a-z]{2,6}[-_ ]?\d{2,5}[a-z]{0,2}\s*[:：-]\s*`)
	title = re.ReplaceAllString(title, "")
	return strings.TrimSpace(title)
}

func normalizeThePornDBCode(value string) string {
	value = strings.ToUpper(strings.TrimSpace(value))
	var b strings.Builder
	for _, r := range value {
		if (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func normalizeThePornDBCodeDisplay(value string) string {
	value = strings.ToUpper(strings.TrimSpace(value))
	if value == "" {
		return ""
	}
	if strings.Contains(value, "-") {
		return value
	}
	if matches := util.CodeRe.FindStringSubmatch(value); len(matches) >= 3 {
		return strings.ToUpper(matches[1]) + "-" + strings.ToUpper(matches[2]) + strings.ToUpper(matches[3])
	}
	return value
}
