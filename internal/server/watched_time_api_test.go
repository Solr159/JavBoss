package server

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
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"javboss/internal/common"
	dbpkg "javboss/internal/db"
	"javboss/internal/models"
)

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
				previousUpdate := updateLANAccess
				t.Cleanup(func() { updateLANAccess = previousUpdate })
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
				listener, err := net.Listen("tcp", "127.0.0.1:0")
				if err != nil {
					t.Fatal(err)
				}
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
	testAuthService(t)
	video := models.Video{Fingerprint: "watch-snapshot", WatchedMS: 123}
	jav := models.Jav{Code: "WATCH-SNAPSHOT", WatchedMS: 456}
	for _, row := range []any{&video, &jav} {
		if err := common.DB.Create(row).Error; err != nil {
			t.Fatal(err)
		}
	}
	updates, unsubscribe := watchedTimeEvents.subscribe()
	defer unsubscribe()
	if err := common.DB.Exec(`CREATE TRIGGER reject_watch BEFORE UPDATE OF watched_ms ON jav BEGIN SELECT RAISE(ABORT, 'failed'); END`).Error; err != nil {
		t.Fatal(err)
	}
	if err := saveWatchedTime(context.Background(), video.ID, jav.ID, 1000); err == nil {
		t.Fatal("expected rollback")
	}
	select {
	case update := <-updates:
		t.Fatalf("published rollback: %+v", update)
	default:
	}
	if err := common.DB.First(&video).Error; err != nil {
		t.Fatal(err)
	}
	if video.WatchedMS != 123 {
		t.Fatal("video increment was not rolled back")
	}
	if err := common.DB.Exec("DROP TRIGGER reject_watch").Error; err != nil {
		t.Fatal(err)
	}
	if err := saveWatchedTime(context.Background(), video.ID, jav.ID, 1000); err != nil {
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
	router := gin.New()
	router.GET("/events", streamWatchedTime)
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
	watchedTimeEvents.publish(expected)
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
	// Cancellation must remove the subscription even without another event.
	deadline := time.Now().Add(time.Second)
	for {
		watchedTimeEvents.mu.Lock()
		count := len(watchedTimeEvents.subscribers)
		watchedTimeEvents.mu.Unlock()
		if count == 0 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("stream subscription leaked")
		}
		time.Sleep(time.Millisecond)
	}
}

func TestWatchedTimeSlowSubscriberReconnects(t *testing.T) {
	hub := &watchedTimeHub{subscribers: make(map[chan dbpkg.WatchedTimeSnapshot]struct{})}
	slow, unsubscribe := hub.subscribe()
	defer unsubscribe()
	for i := 0; i < 33; i++ {
		hub.publish(dbpkg.WatchedTimeSnapshot{})
	}
	if len(hub.subscribers) != 0 {
		t.Fatal("slow subscriber was retained")
	}
	count := 0
	for range slow {
		count++
	}
	if count != 32 {
		t.Fatalf("buffered events: %d", count)
	}
}

func TestWatchedTimeRoutesRequireAuthentication(t *testing.T) {
	router := NewRouter("", testAuthService(t))
	for _, path := range []string{"/videos/watched-time", "/videos/watched-time/events"} {
		response := httptest.NewRecorder()
		router.ServeHTTP(response, httptest.NewRequest("GET", path, nil))
		if response.Code != 401 {
			t.Fatalf("%s: %d", path, response.Code)
		}
	}
}
