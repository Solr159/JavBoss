package update

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestReleaseNotesFromPublicPage(t *testing.T) {
	for _, tc := range []struct {
		name, page, want string
		status           int
		bad              bool
	}{
		{"formatted", `<nav>Ignore navigation</nav><div class="markdown-body">Ignore other markdown</div>
<div data-test-selector="body-content" class="markdown-body tmp-my-3"><h1>更新日志</h1>
<h2>新功能</h2><ul><li>支持 <strong>AI 翻译</strong> &amp; <code>模型配置</code></li><li>修复更新问题</li></ul>
<p>查看 <a href="https://example.com">说明</a><br>第二行</p><script>untrusted()</script><style>hidden</style><svg>icon</svg></div><footer>Ignore footer</footer>`,
			"更新日志\n新功能\n• 支持 AI 翻译 & 模型配置\n• 修复更新问题\n查看 说明\n第二行", 200, false},
		{"empty notes", `<div data-test-selector="body-content" class="markdown-body"></div>`, "", 200, false},
		{"missing notes", `<html>Sign in</html>`, "", 200, true},
		{"forbidden", "", "", 403, true},
		{"too large", strings.Repeat(" ", (4<<20)+1), "", 200, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodGet || r.URL.Path != "/releases/tag/v2.1.3" {
					t.Errorf("unexpected request: %s %s", r.Method, r.URL)
				}
				w.WriteHeader(tc.status)
				_, _ = w.Write([]byte(tc.page))
			}))
			defer server.Close()
			notes, err := readReleaseNotes(t.Context(), server.Client(), server.URL+"/releases/tag/v2.1.3")
			if (err != nil) != tc.bad || notes != tc.want {
				t.Fatalf("notes=%q err=%v", notes, err)
			}
		})
	}
}

func TestUnavailableNotesDoNotBlockInstall(t *testing.T) {
	m := newManager(t.Context(), Options{BaseDir: t.TempDir(), Version: "v1.0.0", Release: true})
	m.platform = "linux-x86_64"
	m.status.Notes = "outdated notes"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodHead && r.URL.Path == "/releases/latest" {
			w.Header().Set("Location", "https://github.com/"+repository+"/releases/tag/v2.1.3")
			w.WriteHeader(http.StatusFound)
			return
		}
		if r.Method != http.MethodGet || r.URL.Path != "/releases/tag/v2.1.3" {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL)
		}
		w.WriteHeader(http.StatusForbidden)
	}))
	defer server.Close()
	m.endpoint = server.URL + "/releases/latest"
	m.client = server.Client()
	m.check()
	status := m.Status()
	if !status.CanInstall || !status.Available || status.Error != "" || status.Notes != "" || status.LatestVersion != "v2.1.3" {
		t.Fatalf("status=%+v", status)
	}
}

func TestReleaseNotesRequestCanBeCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cancel()
		<-r.Context().Done()
	}))
	defer server.Close()
	defer cancel()
	if _, err := readReleaseNotes(ctx, server.Client(), server.URL); err == nil {
		t.Fatal("canceled notes request succeeded")
	}
}
