package openlist

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"javboss/internal/downloader"
)

func TestClientReportsMissingTemporaryDirectory(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/api/me":
			writeOpenListTestResponse(response, http.StatusOK, map[string]any{"username": "admin", "role": 2})
		case "/api/admin/setting/get":
			writeOpenListTestResponse(response, http.StatusOK, map[string]any{"value": ""})
		default:
			writeOpenListTestResponse(response, http.StatusNotFound, nil)
		}
	}))
	defer server.Close()

	client, err := New(server.URL, "admin-token")
	if err != nil {
		t.Fatalf("new client: %v", err)
	}
	defer client.Close()
	if _, err := client.Test(context.Background(), "/115/JavBoss"); !errors.Is(err, ErrTemporaryDirectoryNotConfigured) {
		t.Fatalf("test error = %v, want ErrTemporaryDirectoryNotConfigured", err)
	}
}

func TestClientUses115OpenAndResolvesDownload(t *testing.T) {
	t.Helper()
	var submittedTool string
	var submittedPath string
	var canceledTask string
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.Header.Get("Authorization") != "admin-token" {
			writeOpenListTestResponse(response, http.StatusUnauthorized, nil)
			return
		}
		switch request.URL.Path {
		case "/openlist/api/me":
			writeOpenListTestResponse(response, http.StatusOK, map[string]any{"username": "admin", "role": 2})
		case "/openlist/api/admin/setting/get":
			writeOpenListTestResponse(response, http.StatusOK, map[string]any{"value": "/115/.offline"})
		case "/openlist/api/admin/storage/list":
			writeOpenListTestResponse(response, http.StatusOK, map[string]any{
				"content": []map[string]any{{"mount_path": "/115", "driver": "115 Open", "status": "work", "disabled": false}},
				"total":   1,
			})
		case "/openlist/api/fs/get":
			writeOpenListTestResponse(response, http.StatusOK, map[string]any{"name": "JavBoss", "is_dir": true})
		case "/openlist/api/fs/add_offline_download":
			var body struct {
				URLs         []string `json:"urls"`
				Path         string   `json:"path"`
				Tool         string   `json:"tool"`
				DeletePolicy string   `json:"delete_policy"`
			}
			if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
				t.Errorf("decode offline request: %v", err)
			}
			submittedTool = body.Tool
			submittedPath = body.Path
			if len(body.URLs) != 1 || !strings.HasPrefix(body.URLs[0], "magnet:") || body.DeletePolicy != "delete_never" {
				t.Errorf("unexpected offline request: %+v", body)
			}
			writeOpenListTestResponse(response, http.StatusOK, map[string]any{"tasks": []map[string]any{{"id": "task-1"}}})
		case "/openlist/api/task/offline_download/info":
			if request.URL.Query().Get("tid") == "duplicate-task" {
				writeOpenListTestResponse(response, http.StatusOK, map[string]any{
					"id": "duplicate-task", "state": 7, "status": "failed",
					"error": "failed to add offline download task: code: 10008, message: 任务已存在，请勿输入重复的链接地址",
				})
				return
			}
			writeOpenListTestResponse(response, http.StatusOK, map[string]any{"id": "task-1", "state": 2, "status": "done"})
		case "/openlist/api/fs/list":
			var body struct {
				Path string `json:"path"`
			}
			_ = json.NewDecoder(request.Body).Decode(&body)
			if body.Path == "/115/JavBoss/job" {
				writeOpenListTestResponse(response, http.StatusOK, map[string]any{
					"content": []map[string]any{{"name": "disc", "is_dir": true}, {"name": "cover.jpg", "size": 12}}, "total": 2,
				})
				return
			}
			writeOpenListTestResponse(response, http.StatusOK, map[string]any{
				"content": []map[string]any{{"name": "movie.mp4", "size": 1024}}, "total": 1,
			})
		case "/openlist/api/fs/link":
			writeOpenListTestResponse(response, http.StatusOK, map[string]any{
				"url":    serverURLFromRequest(request) + "/content/movie.mp4",
				"header": map[string][]string{"Cookie": {"UID=123"}, "Authorization": {"must-not-leak"}},
			})
		case "/openlist/api/task/offline_download/cancel":
			canceledTask = request.URL.Query().Get("tid")
			writeOpenListTestResponse(response, http.StatusOK, nil)
		default:
			t.Errorf("unexpected OpenList request: %s %s", request.Method, request.URL.String())
			writeOpenListTestResponse(response, http.StatusNotFound, nil)
		}
	}))
	defer server.Close()

	client, err := New(server.URL+"/openlist", "admin-token")
	if err != nil {
		t.Fatalf("new client: %v", err)
	}
	defer client.Close()
	ctx := context.Background()
	testResult, err := client.Test(ctx, "/115/JavBoss")
	if err != nil {
		t.Fatalf("test OpenList: %v", err)
	}
	if testResult.Provider != "openlist" || testResult.UserName != "admin" {
		t.Fatalf("unexpected test result: %+v", testResult)
	}
	taskID, err := client.StartOffline(ctx, "magnet:?xt=urn:btih:abc", "/115/JavBoss/job", "abc")
	if err != nil || taskID != "task-1" {
		t.Fatalf("start offline task ID=%q err=%v", taskID, err)
	}
	if submittedTool != "115 Open" || submittedPath != "/115/JavBoss/job" {
		t.Fatalf("offline tool=%q path=%q", submittedTool, submittedPath)
	}
	status, err := client.OfflineStatus(ctx, taskID, "", "")
	if err != nil || status.State != downloader.OfflineComplete {
		t.Fatalf("offline status=%+v err=%v", status, err)
	}
	duplicateStatus, err := client.OfflineStatus(ctx, "duplicate-task", "", "")
	if err != nil || duplicateStatus.State != downloader.OfflineUntracked {
		t.Fatalf("duplicate offline status=%+v err=%v", duplicateStatus, err)
	}
	files, err := client.WalkFiles(ctx, "/115/JavBoss/job")
	if err != nil || len(files) != 2 || files[1].Path != "/115/JavBoss/job/disc/movie.mp4" {
		t.Fatalf("walk files=%+v err=%v", files, err)
	}
	source, err := client.DownloadSource(ctx, files[1].Path)
	if err != nil || source.Headers.Get("Cookie") != "UID=123" || source.Headers.Get("Authorization") != "" {
		t.Fatalf("download source=%+v err=%v", source, err)
	}
	if err := client.CancelOffline(ctx, taskID); err != nil || canceledTask != taskID {
		t.Fatalf("cancel task=%q err=%v", canceledTask, err)
	}
}

func TestDuplicate115OfflineTaskError(t *testing.T) {
	tests := []struct {
		message string
		want    bool
	}{
		{message: "failed to add offline download task: code: 10008, message: task already exists", want: true},
		{message: "failed to add offline download task: code=10008", want: true},
		{message: "任务已存在，请勿输入重复的链接地址", want: true},
		{message: "failed to add offline download task: code: 10009", want: false},
	}
	for _, test := range tests {
		if got := isDuplicate115OfflineTaskError(test.message); got != test.want {
			t.Errorf("isDuplicate115OfflineTaskError(%q) = %t, want %t", test.message, got, test.want)
		}
	}
}

func TestClientRejectsNon115OpenTarget(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/api/me":
			writeOpenListTestResponse(response, http.StatusOK, map[string]any{"username": "admin", "role": 2})
		case "/api/admin/setting/get":
			writeOpenListTestResponse(response, http.StatusOK, map[string]any{"value": "/115/.offline"})
		case "/api/admin/storage/list":
			writeOpenListTestResponse(response, http.StatusOK, map[string]any{
				"content": []map[string]any{
					{"mount_path": "/115", "driver": "115 Open", "status": "work"},
					{"mount_path": "/other", "driver": "Local", "status": "work"},
				},
				"total": 2,
			})
		default:
			writeOpenListTestResponse(response, http.StatusNotFound, nil)
		}
	}))
	defer server.Close()
	client, err := New(server.URL, "admin-token")
	if err != nil {
		t.Fatalf("new client: %v", err)
	}
	defer client.Close()
	if _, err := client.Test(context.Background(), "/other/JavBoss"); err == nil || !strings.Contains(err.Error(), "115 Open") {
		t.Fatalf("expected 115 Open validation error, got %v", err)
	}
}

func writeOpenListTestResponse(response http.ResponseWriter, status int, data any) {
	response.Header().Set("Content-Type", "application/json")
	response.WriteHeader(status)
	_ = json.NewEncoder(response).Encode(map[string]any{"code": status, "message": http.StatusText(status), "data": data})
}

func serverURLFromRequest(request *http.Request) string {
	return "http://" + request.Host
}
