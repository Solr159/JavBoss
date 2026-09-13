package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"javboss/internal/common"
	"javboss/internal/db"
	"javboss/internal/models"
	"javboss/internal/service"
	"javboss/internal/util"
)

func TestDirectoryTranscodeAPIProgressAndBusyProtection(t *testing.T) {
	if _, err := util.ResolveFFmpegPath(); err != nil {
		t.Skip("ffmpeg unavailable")
	}
	if _, err := util.ResolveFFprobePath(); err != nil {
		t.Skip("ffprobe unavailable")
	}
	database, err := db.Open(filepath.Join(t.TempDir(), "api.db"))
	if err != nil {
		t.Fatal(err)
	}
	previous := common.DB
	common.DB = database
	t.Cleanup(func() { common.DB = previous; sqlDB, _ := database.DB(); _ = sqlDB.Close() })
	directory := models.Directory{Path: t.TempDir()}
	if err := database.Create(&directory).Error; err != nil {
		t.Fatal(err)
	}
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.POST("/directories/:id/process", processDirectory)
	router.GET("/directories", listDirectories)
	router.DELETE("/directories/:id/transcode", cancelDirectoryTranscode)
	path := "/directories/" + strconv.FormatInt(directory.ID, 10)
	send := func(method, path, body string) *httptest.ResponseRecorder {
		t.Helper()
		response := httptest.NewRecorder()
		request := httptest.NewRequest(method, path, strings.NewReader(body))
		request.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(response, request)
		return response
	}
	release, err := service.CancelAndReserveDirectoryScan(t.Context(), directory.ID)
	if err != nil {
		t.Fatal(err)
	}
	response := send(http.MethodPost, path+"/process", `{"mode":"transcode"}`)
	release()
	if response.Code != http.StatusConflict {
		t.Fatalf("busy directory accepted job: %d %s", response.Code, response.Body.String())
	}
	response = send(http.MethodPost, path+"/process", `{"mode":"transcode"}`)
	if response.Code != http.StatusAccepted {
		t.Fatalf("start: %d %s", response.Code, response.Body.String())
	}
	deadline := time.Now().Add(5 * time.Second)
	for service.DirectoryWorkStatus(directory.ID) != service.DirectoryWorkIdle && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}
	if service.DirectoryWorkStatus(directory.ID) != service.DirectoryWorkIdle {
		service.CancelDirectoryTranscode(directory.ID)
		t.Fatal("empty-directory job did not finish")
	}
	response = send(http.MethodGet, "/directories", "")
	if response.Code != http.StatusOK {
		t.Fatal(response.Body.String())
	}
	var directories []struct {
		TranscodeProgress *service.DirectoryTranscodeProgress `json:"transcode_progress"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &directories); err != nil {
		t.Fatal(err)
	}
	if len(directories) != 1 || directories[0].TranscodeProgress == nil || directories[0].TranscodeProgress.Phase != "completed" || directories[0].TranscodeProgress.Total != 0 {
		t.Fatalf("progress missing: %s", response.Body.String())
	}
	response = send(http.MethodDelete, path+"/transcode", "")
	if response.Code != http.StatusConflict {
		t.Fatalf("cancel idle job: %d", response.Code)
	}
	response = send(http.MethodDelete, "/directories/invalid/transcode", "")
	if response.Code != http.StatusBadRequest {
		t.Fatalf("invalid ID: %d", response.Code)
	}
	// File mutation reservations must conflict with a running directory task.
	release, err = service.CancelAndReserveDirectoryScan(context.Background(), directory.ID)
	if err != nil {
		t.Fatal(err)
	}
	defer release()
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPatch, "/", nil)
	if _, ok := reserveVideoFileMutation(c, directory.ID); ok || recorder.Code != http.StatusConflict {
		t.Fatal("file mutation bypassed task reservation")
	}
}
