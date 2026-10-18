package api

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"net/url"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	dbpkg "javboss/internal/db"
	"javboss/internal/models"
	"javboss/internal/playback"

	"github.com/gin-gonic/gin"
)

type closeCountingListener struct {
	net.Listener
	closes atomic.Int32
}

func (l *closeCountingListener) Close() error {
	l.closes.Add(1)
	return l.Listener.Close()
}

func TestHTTPShutdownEndsWatchedTimeStreams(t *testing.T) {
	for _, proxy := range []bool{false, true} {
		for _, listenerFailure := range []bool{false, true} {
			name := "server"
			if proxy {
				name = "client-proxy"
			}
			if listenerFailure {
				name += "/listener-failure"
			} else {
				name += "/exit"
			}
			t.Run(name, func(t *testing.T) {

				streamDone := make(chan struct{})
				router := gin.New()
				router.GET("/events", func(c *gin.Context) {
					defer close(streamDone)
					streamWatchedTime(c)
				})
				var handler http.Handler = router
				if proxy {
					upstream := httptest.NewServer(router)
					defer upstream.Close()
					target, err := url.Parse(upstream.URL)
					if err != nil {
						t.Fatal(err)
					}
					handler = httputil.NewSingleHostReverseProxy(target)
				}
				tcpListener, err := net.Listen("tcp", "127.0.0.1:0")
				if err != nil {
					t.Fatal(err)
				}
				listener := &closeCountingListener{Listener: tcpListener}
				srv := &http.Server{Handler: handler}
				defer srv.Close()
				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()
				done := make(chan error, 1)
				go func() { done <- ServeHTTP(ctx, srv, listener, false, false) }()
				client := &http.Client{Timeout: 5 * time.Second}
				defer client.CloseIdleConnections()
				response, err := client.Get("http://" + listener.Addr().String() + "/events")
				if err != nil {
					t.Fatal(err)
				}
				defer response.Body.Close()
				if response.StatusCode != http.StatusOK || response.Header.Get("Content-Type") != "text/event-stream" {
					t.Fatalf("stream response: %s %v", response.Status, response.Header)
				}
				// Keep the browser's stream open while the application exits.
				if listenerFailure {
					if err := listener.Close(); err != nil {
						t.Fatal(err)
					}
				} else {
					cancel()
				}
				select {
				case err := <-done:
					if listenerFailure {
						if !errors.Is(err, net.ErrClosed) || errors.Is(err, context.DeadlineExceeded) {
							t.Fatalf("listener failure returned %v", err)
						}
					} else if err != nil {
						t.Fatalf("shutdown: %v", err)
					}
				case <-time.After(3 * time.Second):
					t.Fatal("shutdown waited for the browser to disconnect")
				}
				if !listenerFailure && listener.closes.Load() != 1 {
					t.Fatalf("graceful shutdown closed the listener %d times; want 1", listener.closes.Load())
				}
				select {
				case <-streamDone:
				case <-time.After(time.Second):
					t.Fatal("watched-time stream survived shutdown")
				}
			})
		}
	}
}

func TestWatchedTimeSnapshotAndRollback(t *testing.T) {
	deps := newTestDependencies()
	testAuthService(deps, t)
	video := models.Video{Fingerprint: "watch-snapshot", WatchedMS: 123}
	jav := models.Jav{Code: "WATCH-SNAPSHOT", WatchedMS: 456}
	for _, row := range []any{&video, &jav} {
		if err := deps.database.Create(row).Error; err != nil {
			t.Fatal(err)
		}
	}
	updates, unsubscribe := playback.SubscribeWatchedTime()
	defer unsubscribe()
	if err := deps.database.Exec(`CREATE TRIGGER reject_watch BEFORE UPDATE OF watched_ms ON jav BEGIN SELECT RAISE(ABORT, 'failed'); END`).Error; err != nil {
		t.Fatal(err)
	}
	if err := playback.SaveWatchedTime(context.Background(), video.ID, jav.ID, 1000); err == nil {
		t.Fatal("expected rollback")
	}
	select {
	case update := <-updates:
		t.Fatalf("published rollback: %+v", update)
	default:
	}
	if err := deps.database.First(&video).Error; err != nil {
		t.Fatal(err)
	}
	if video.WatchedMS != 123 {
		t.Fatal("video increment was not rolled back")
	}
	if err := deps.database.Exec("DROP TRIGGER reject_watch").Error; err != nil {
		t.Fatal(err)
	}
	if err := playback.SaveWatchedTime(context.Background(), video.ID, jav.ID, 1000); err != nil {
		t.Fatal(err)
	}
	expected := <-updates
	router := gin.New()
	router.GET("/videos/watched-time", getWatchedTime)
	for _, query := range []string{"video_ids=0", "video_ids=-1", "jav_ids=no", "video_ids=1,", "video_ids=" + strings.Repeat("1,", 200) + "1"} {
		response := httptest.NewRecorder()
		router.ServeHTTP(response, httptest.NewRequest("GET", "/videos/watched-time?"+query, nil))
		if response.Code != 400 {
			t.Fatalf("%s: status %d", query, response.Code)
		}
	}
	response := httptest.NewRecorder()
	// IDs start at 1 in the isolated test database. Missing rows are omitted.
	router.ServeHTTP(response, httptest.NewRequest("GET", "/videos/watched-time?video_ids=1,9999&jav_ids=1,9999", nil))
	var actual dbpkg.WatchedTimeSnapshot
	if err := json.Unmarshal(response.Body.Bytes(), &actual); err != nil {
		t.Fatal(err)
	}
	if response.Code != 200 || !reflect.DeepEqual(actual, expected) || actual.Videos[0].WatchedMS != 1123 || actual.Javs[0].WatchedMS != 1456 {
		t.Fatalf("snapshot: %d %+v; expected %+v", response.Code, actual, expected)
	}
}

func TestWatchedTimeStreamSurvivesResponseTimeout(t *testing.T) {
	deps := newTestDependencies()
	testAuthService(deps, t)
	video := models.Video{ID: 42, Fingerprint: "stream-timeout"}
	if err := deps.database.Create(&video).Error; err != nil {
		t.Fatal(err)
	}
	router := gin.New()
	streamDone := make(chan struct{})
	router.GET("/events", func(c *gin.Context) { streamWatchedTime(c); close(streamDone) })
	server := httptest.NewUnstartedServer(router)
	server.Config.WriteTimeout = 20 * time.Millisecond
	server.Start()
	defer server.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	request, _ := http.NewRequestWithContext(ctx, "GET", server.URL+"/events", nil)
	response, err := server.Client().Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.Header.Get("Content-Type") != "text/event-stream" {
		t.Fatal(response.Header)
	}
	reader := bufio.NewReader(response.Body)
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			t.Fatal(err)
		}
		if line == "\n" {
			break
		}
	}
	time.Sleep(40 * time.Millisecond)
	expected := dbpkg.WatchedTimeSnapshot{Videos: []dbpkg.WatchedTimeTotal{{ID: 42, WatchedMS: 19000}}, Javs: []dbpkg.WatchedTimeTotal{}}
	if err := playback.SaveWatchedTime(ctx, video.ID, 0, 19000); err != nil {
		t.Fatal(err)
	}
	line, err := reader.ReadString('\n')
	if err != nil || line != "event: watched-time\n" {
		t.Fatalf("event: %q %v", line, err)
	}
	line, err = reader.ReadString('\n')
	if err != nil {
		t.Fatal(err)
	}
	var actual dbpkg.WatchedTimeSnapshot
	if err := json.Unmarshal([]byte(strings.TrimPrefix(line, "data: ")), &actual); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(actual, expected) {
		t.Fatalf("event: %+v", actual)
	}
	cancel()
	// The handler returns only after releasing its subscription.
	select {
	case <-streamDone:
	case <-time.After(time.Second):
		t.Fatal("stream subscription leaked")
	}

}

func TestWatchedTimeRoutesRequireAuthentication(t *testing.T) {
	deps := newTestDependencies()
	router := NewRouter("", testAuthService(deps, t))
	for _, path := range []string{"/videos/watched-time", "/videos/watched-time/events"} {
		response := httptest.NewRecorder()
		router.ServeHTTP(response, httptest.NewRequest("GET", path, nil))
		if response.Code != 401 {
			t.Fatalf("%s: %d", path, response.Code)
		}
	}
}
