package openlist

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"strings"

	"javboss/internal/downloader"
	"javboss/internal/util"
)

const (
	providerName      = "openlist"
	offlineTool       = "115 Open"
	maxResponseBytes  = 16 << 20
	maxDirectoryItems = 10000
)

var ErrTemporaryDirectoryNotConfigured = errors.New("OpenList 115 Open temporary directory is not configured")

type Client struct {
	baseURL *url.URL
	token   string
	http    *http.Client
}

type apiError struct {
	Status  int
	Code    int
	Message string
}

func (e *apiError) Error() string {
	if e.Message != "" {
		return fmt.Sprintf("OpenList API error: %s", e.Message)
	}
	return fmt.Sprintf("OpenList API error: HTTP %d, code %d", e.Status, e.Code)
}

type apiEnvelope struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data"`
}

type object struct {
	Name  string `json:"name"`
	Size  int64  `json:"size"`
	IsDir bool   `json:"is_dir"`
}

type listData struct {
	Content []object `json:"content"`
	Total   int      `json:"total"`
}

type storage struct {
	MountPath string `json:"mount_path"`
	Driver    string `json:"driver"`
	Status    string `json:"status"`
	Disabled  bool   `json:"disabled"`
}

type storageListData struct {
	Content []storage `json:"content"`
	Total   int       `json:"total"`
}

type taskInfo struct {
	ID     string `json:"id"`
	State  int    `json:"state"`
	Status string `json:"status"`
	Error  string `json:"error"`
}

type linkData struct {
	URL    string      `json:"url"`
	Header http.Header `json:"header"`
}

func New(address, token string) (*Client, error) {
	address = strings.TrimSpace(address)
	if address == "" {
		return nil, errors.New("OpenList address is required")
	}
	if !strings.Contains(address, "://") {
		address = "http://" + address
	}
	baseURL, err := url.Parse(address)
	if err != nil || baseURL.Host == "" || (baseURL.Scheme != "http" && baseURL.Scheme != "https") {
		return nil, errors.New("OpenList address must be an http or https URL")
	}
	baseURL.RawQuery = ""
	baseURL.Fragment = ""
	baseURL.Path = strings.TrimRight(baseURL.Path, "/") + "/"
	client := util.NewHTTPClientWithTransport(0, func(transport *http.Transport) {
		transport.MaxIdleConns = 20
		transport.MaxIdleConnsPerHost = 4
	})
	return &Client{baseURL: baseURL, token: strings.TrimSpace(token), http: client}, nil
}

func (c *Client) Close() error {
	c.http.CloseIdleConnections()
	return nil
}

func (c *Client) Test(ctx context.Context, folder string) (*downloader.TestResult, error) {
	var user struct {
		Username string `json:"username"`
		Role     int    `json:"role"`
	}
	if err := c.doJSON(ctx, http.MethodGet, "api/me", nil, &user); err != nil {
		return nil, fmt.Errorf("authenticate OpenList: %w", err)
	}
	if user.Role != 2 {
		return nil, errors.New("OpenList administrator API token is required")
	}
	var setting struct {
		Value string `json:"value"`
	}
	if err := c.doJSON(ctx, http.MethodGet, "api/admin/setting/get?key=115_open_temp_dir", nil, &setting); err != nil {
		return nil, fmt.Errorf("read OpenList 115 Open settings: %w", err)
	}
	tempFolder := cleanPath(setting.Value)
	if strings.TrimSpace(setting.Value) == "" {
		return nil, ErrTemporaryDirectoryNotConfigured
	}
	storages, err := c.listStorages(ctx)
	if err != nil {
		return nil, err
	}
	targetStorage := storageForPath(storages, folder)
	if err := validate115OpenStorage(targetStorage, "target folder"); err != nil {
		return nil, err
	}
	tempStorage := storageForPath(storages, tempFolder)
	if err := validate115OpenStorage(tempStorage, "115 Open temporary directory"); err != nil {
		return nil, err
	}
	remoteFolder := cleanPath(folder)
	obj, err := c.getObject(ctx, remoteFolder)
	if err != nil {
		return nil, fmt.Errorf("get OpenList target folder: %w", err)
	}
	if !obj.IsDir {
		return nil, errors.New("OpenList target path is not a directory")
	}
	return &downloader.TestResult{
		Provider: providerName, UserName: user.Username, Folder: remoteFolder,
		Details: map[string]any{
			"driver": offlineTool, "mount_path": targetStorage.MountPath,
			"temp_folder": tempFolder,
		},
	}, nil
}

func (c *Client) EnsureFolder(ctx context.Context, parent, name string) (string, error) {
	fullPath := cleanPath(path.Join(parent, name))
	if obj, err := c.getObject(ctx, fullPath); err == nil {
		if !obj.IsDir {
			return "", fmt.Errorf("OpenList path %s is not a directory", fullPath)
		}
		return fullPath, nil
	}
	request := map[string]string{"path": fullPath}
	if err := c.doJSON(ctx, http.MethodPost, "api/fs/mkdir", request, nil); err != nil {
		if obj, getErr := c.getObject(ctx, fullPath); getErr == nil && obj.IsDir {
			return fullPath, nil
		}
		return "", err
	}
	return fullPath, nil
}

func (c *Client) StartOffline(ctx context.Context, magnet, folder, _ string) (string, error) {
	request := map[string]any{
		"urls": []string{magnet}, "path": cleanPath(folder),
		"tool": offlineTool, "delete_policy": "delete_never",
	}
	var data struct {
		Tasks []taskInfo `json:"tasks"`
	}
	if err := c.doJSON(ctx, http.MethodPost, "api/fs/add_offline_download", request, &data); err != nil {
		return "", err
	}
	if len(data.Tasks) == 0 || strings.TrimSpace(data.Tasks[0].ID) == "" {
		return "", errors.New("OpenList returned no offline task ID")
	}
	return strings.TrimSpace(data.Tasks[0].ID), nil
}

func (c *Client) OfflineStatus(ctx context.Context, taskID, _ string, _ string) (*downloader.OfflineStatus, error) {
	if strings.TrimSpace(taskID) == "" {
		return &downloader.OfflineStatus{State: downloader.OfflineNotFound}, nil
	}
	var task taskInfo
	endpoint := "api/task/offline_download/info?tid=" + url.QueryEscape(taskID)
	if err := c.doJSON(ctx, http.MethodPost, endpoint, nil, &task); err != nil {
		var apiErr *apiError
		if errors.As(err, &apiErr) && (apiErr.Status == http.StatusNotFound || apiErr.Code == http.StatusNotFound) {
			return &downloader.OfflineStatus{State: downloader.OfflineNotFound}, nil
		}
		return nil, err
	}
	status := &downloader.OfflineStatus{State: downloader.OfflineRunning, Message: task.Status}
	switch task.State {
	case 2:
		status.State = downloader.OfflineComplete
	case 3, 4:
		status.State = downloader.OfflineCanceled
	case 7:
		if isDuplicate115OfflineTaskError(task.Error + " " + task.Status) {
			status.State = downloader.OfflineUntracked
			status.Message = task.Error
			break
		}
		status.State = downloader.OfflineFailed
		if task.Error != "" {
			status.Message = task.Error
		}
	}
	return status, nil
}

func isDuplicate115OfflineTaskError(message string) bool {
	normalized := strings.ToLower(strings.TrimSpace(message))
	return strings.Contains(normalized, "code: 10008") ||
		strings.Contains(normalized, "code=10008") ||
		strings.Contains(message, "任务已存在")
}

func (c *Client) WalkFiles(ctx context.Context, folder string) ([]downloader.RemoteFile, error) {
	queue := []string{cleanPath(folder)}
	result := make([]downloader.RemoteFile, 0)
	entryCount := 0
	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]
		objects, err := c.list(ctx, current)
		if err != nil {
			return nil, err
		}
		entryCount += len(objects)
		if entryCount > maxDirectoryItems {
			return nil, errors.New("OpenList job directory contains too many entries")
		}
		for _, object := range objects {
			objectPath := cleanPath(path.Join(current, object.Name))
			if object.IsDir {
				queue = append(queue, objectPath)
			} else {
				result = append(result, downloader.RemoteFile{Path: objectPath, Name: object.Name, Size: object.Size})
			}
		}
	}
	return result, nil
}

func (c *Client) DownloadSource(ctx context.Context, remotePath string) (*downloader.DownloadSource, error) {
	var link linkData
	if err := c.doJSON(ctx, http.MethodPost, "api/fs/link", map[string]string{"path": cleanPath(remotePath)}, &link); err != nil {
		return nil, err
	}
	rawURL := strings.TrimSpace(link.URL)
	if rawURL == "" {
		return nil, errors.New("OpenList returned no download URL")
	}
	resolved, err := c.baseURL.Parse(rawURL)
	if err != nil || resolved.Host == "" || (resolved.Scheme != "http" && resolved.Scheme != "https") {
		return nil, errors.New("OpenList returned an invalid download URL")
	}
	headers := link.Header.Clone()
	headers.Del("Authorization")
	headers.Del("Host")
	headers.Del("Content-Length")
	return &downloader.DownloadSource{URL: resolved.String(), Headers: headers}, nil
}

func (c *Client) CancelOffline(ctx context.Context, taskID string) error {
	if strings.TrimSpace(taskID) == "" {
		return nil
	}
	endpoint := "api/task/offline_download/cancel?tid=" + url.QueryEscape(taskID)
	return c.doJSON(ctx, http.MethodPost, endpoint, nil, nil)
}

func (c *Client) getObject(ctx context.Context, remotePath string) (*object, error) {
	var result object
	if err := c.doJSON(ctx, http.MethodPost, "api/fs/get", map[string]string{"path": cleanPath(remotePath)}, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

func (c *Client) list(ctx context.Context, folder string) ([]object, error) {
	result := make([]object, 0)
	for pageNumber := 1; ; pageNumber++ {
		var data listData
		request := map[string]any{
			"path": cleanPath(folder), "refresh": true, "page": pageNumber, "per_page": 100,
		}
		if err := c.doJSON(ctx, http.MethodPost, "api/fs/list", request, &data); err != nil {
			return nil, err
		}
		result = append(result, data.Content...)
		if len(result) >= data.Total || len(data.Content) == 0 {
			return result, nil
		}
		if len(result) > maxDirectoryItems {
			return nil, errors.New("OpenList directory contains too many entries")
		}
	}
}

func (c *Client) listStorages(ctx context.Context) ([]storage, error) {
	result := make([]storage, 0)
	for pageNumber := 1; ; pageNumber++ {
		var data storageListData
		endpoint := "api/admin/storage/list?page=" + strconv.Itoa(pageNumber) + "&per_page=100"
		if err := c.doJSON(ctx, http.MethodGet, endpoint, nil, &data); err != nil {
			return nil, fmt.Errorf("list OpenList storages: %w", err)
		}
		result = append(result, data.Content...)
		if len(result) >= data.Total || len(data.Content) == 0 {
			return result, nil
		}
	}
}

func (c *Client) doJSON(ctx context.Context, method, endpoint string, requestBody any, responseData any) error {
	var body io.Reader
	if requestBody != nil {
		encoded, err := json.Marshal(requestBody)
		if err != nil {
			return err
		}
		body = bytes.NewReader(encoded)
	}
	endpointURL, err := c.endpointURL(endpoint)
	if err != nil {
		return err
	}
	request, err := http.NewRequestWithContext(ctx, method, endpointURL, body)
	if err != nil {
		return err
	}
	request.Header.Set("Accept", "application/json")
	request.Header.Set("Authorization", c.token)
	if requestBody != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	response, err := c.http.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	var envelope apiEnvelope
	decoder := json.NewDecoder(io.LimitReader(response.Body, maxResponseBytes))
	if err := decoder.Decode(&envelope); err != nil {
		return fmt.Errorf("decode OpenList response: %w", err)
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 || envelope.Code < 200 || envelope.Code >= 300 {
		return &apiError{Status: response.StatusCode, Code: envelope.Code, Message: strings.TrimSpace(envelope.Message)}
	}
	if responseData != nil && len(envelope.Data) > 0 && string(envelope.Data) != "null" {
		if err := json.Unmarshal(envelope.Data, responseData); err != nil {
			return fmt.Errorf("decode OpenList response data: %w", err)
		}
	}
	return nil
}

func (c *Client) endpointURL(endpoint string) (string, error) {
	reference, err := url.Parse(strings.TrimLeft(endpoint, "/"))
	if err != nil {
		return "", err
	}
	return c.baseURL.ResolveReference(reference).String(), nil
}

func storageForPath(storages []storage, remotePath string) *storage {
	remotePath = cleanPath(remotePath)
	var matched *storage
	matchedLength := -1
	for index := range storages {
		mountPath := cleanPath(storages[index].MountPath)
		contains := mountPath == "/" || remotePath == mountPath || strings.HasPrefix(remotePath, mountPath+"/")
		if contains && len(mountPath) > matchedLength {
			matched = &storages[index]
			matchedLength = len(mountPath)
		}
	}
	return matched
}

func validate115OpenStorage(value *storage, label string) error {
	if value == nil {
		return fmt.Errorf("OpenList %s is not inside a configured storage", label)
	}
	if value.Driver != offlineTool {
		return fmt.Errorf("OpenList %s must use the 115 Open driver, got %s", label, value.Driver)
	}
	if value.Disabled || !strings.EqualFold(value.Status, "work") {
		return fmt.Errorf("OpenList 115 Open storage %s is not available", value.MountPath)
	}
	return nil
}

func cleanPath(value string) string {
	value = strings.TrimSpace(strings.ReplaceAll(value, "\\", "/"))
	if value == "" {
		return "/"
	}
	return path.Clean("/" + strings.TrimLeft(value, "/"))
}

var _ downloader.Client = (*Client)(nil)
