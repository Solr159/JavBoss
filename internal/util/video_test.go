package util

import (
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestIsVideoRecognizesMPEGTransportStreamWithMP4Extension(t *testing.T) {
	path := filepath.Join(t.TempDir(), "SNIS-974.mp4")
	content := makeMPEGTransportStreamHeader(188, 0)
	if err := os.WriteFile(path, content, 0o600); err != nil {
		t.Fatalf("write MPEG-TS fixture: %v", err)
	}

	if !IsVideo(path) {
		t.Fatal("IsVideo should recognize MPEG-TS content with an .mp4 extension")
	}
}

func TestIsVideoRecognizesCommonMPEGTransportStreamLayouts(t *testing.T) {
	tests := []struct {
		name       string
		packetSize int
		syncOffset int
	}{
		{name: "TS", packetSize: 188, syncOffset: 0},
		{name: "M2TS", packetSize: 192, syncOffset: 4},
		{name: "192 byte TS", packetSize: 192, syncOffset: 0},
		{name: "204 byte TS", packetSize: 204, syncOffset: 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "misnamed.data")
			content := makeMPEGTransportStreamHeader(tt.packetSize, tt.syncOffset)
			if err := os.WriteFile(path, content, 0o600); err != nil {
				t.Fatalf("write MPEG-TS fixture: %v", err)
			}

			if !IsVideo(path) {
				t.Fatalf("IsVideo should recognize packet size %d with sync offset %d", tt.packetSize, tt.syncOffset)
			}
		})
	}
}

func TestIsVideoRecognizesGenericISOBMFFBrandsWithVideoExtensions(t *testing.T) {
	tests := []struct {
		name  string
		brand string
		ext   string
	}{
		{name: "VR MP4", brand: "vr1d", ext: ".mp4"},
		{name: "QuickTime MOV", brand: "qt  ", ext: ".MOV"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "sample"+tt.ext)
			content := make([]byte, 28)
			binary.BigEndian.PutUint32(content[:4], uint32(len(content)))
			copy(content[4:8], "ftyp")
			copy(content[8:12], tt.brand)
			if err := os.WriteFile(path, content, 0o600); err != nil {
				t.Fatalf("write ISO-BMFF fixture: %v", err)
			}

			if !IsVideo(path) {
				t.Fatalf("IsVideo should recognize ISO-BMFF brand %q", tt.brand)
			}
		})
	}
}

func TestIsVideoCandidateUsesKnownExtensionWithoutAcceptingText(t *testing.T) {
	dir := t.TempDir()
	ogvPath := filepath.Join(dir, "sample.ogv")
	if err := os.WriteFile(ogvPath, []byte("candidate validated later by ffprobe"), 0o600); err != nil {
		t.Fatalf("write OGV candidate: %v", err)
	}
	if !IsVideoCandidate(ogvPath) {
		t.Fatal("known video extension should be accepted as an ffprobe candidate")
	}

	textPath := filepath.Join(dir, "sample.txt")
	if err := os.WriteFile(textPath, []byte("ordinary text"), 0o600); err != nil {
		t.Fatalf("write text fixture: %v", err)
	}
	if IsVideoCandidate(textPath) {
		t.Fatal("ordinary text should not be accepted as an ffprobe candidate")
	}
}

func TestIsVideoRecognizesRMVBRealMediaSignature(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sample.rmvb")
	if err := os.WriteFile(path, append([]byte(".RMF\x00\x00\x00\x12"), make([]byte, 32)...), 0o644); err != nil {
		t.Fatalf("write rmvb fixture: %v", err)
	}

	if !IsVideo(path) {
		t.Fatal("IsVideo should accept rmvb files with a RealMedia signature")
	}
}

func TestIsVideoRejectsRMVBWithoutRealMediaSignature(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sample.rmvb")
	if err := os.WriteFile(path, []byte("not a realmedia file"), 0o644); err != nil {
		t.Fatalf("write rmvb fixture: %v", err)
	}

	if IsVideo(path) {
		t.Fatal("IsVideo should reject rmvb files without a RealMedia signature")
	}
}

func makeMPEGTransportStreamHeader(packetSize, syncOffset int) []byte {
	const packetCount = 4
	content := make([]byte, syncOffset+packetCount*packetSize)
	for packet := 0; packet < packetCount; packet++ {
		content[syncOffset+packet*packetSize] = 0x47
	}
	return content
}

func TestDetectContainerRecognizesRMVBExtension(t *testing.T) {
	if got := detectContainer("rm", "/videos/sample.rmvb"); got != "rmvb" {
		t.Fatalf("detectContainer() = %q, want %q", got, "rmvb")
	}
}

func TestFindFFmpegPathUsesPersistentDataTool(t *testing.T) {
	t.Setenv("JAVBOSS_BUILD_MODE", "development")
	originalWorkingDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("get working directory: %v", err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(originalWorkingDir); err != nil {
			t.Errorf("restore working directory: %v", err)
		}
	})

	baseDir := t.TempDir()
	if err := os.Chdir(baseDir); err != nil {
		t.Fatalf("change working directory: %v", err)
	}
	t.Setenv("JAVBOSS_CONTAINER", "")
	t.Setenv("JAVBOSS_DOCKER", "")

	ignoredEnvPath := filepath.Join(baseDir, "ignored-env-ffmpeg")
	if err := os.WriteFile(ignoredEnvPath, []byte("ignored ffmpeg"), 0o755); err != nil {
		t.Fatalf("write ignored environment FFmpeg fixture: %v", err)
	}
	t.Setenv("FFMPEG_PATH", ignoredEnvPath)

	ffmpegPath := filepath.Join(baseDir, FFmpegToolRelativePath())
	if err := os.MkdirAll(filepath.Dir(ffmpegPath), 0o755); err != nil {
		t.Fatalf("create FFmpeg directory: %v", err)
	}
	if err := os.WriteFile(ffmpegPath, []byte("test ffmpeg"), 0o755); err != nil {
		t.Fatalf("write FFmpeg fixture: %v", err)
	}

	got, err := findFFmpegPath()
	if err != nil {
		t.Fatalf("find FFmpeg: %v", err)
	}
	if filepath.Clean(got) != filepath.Clean(ffmpegPath) {
		t.Fatalf("findFFmpegPath() = %q, want %q", got, ffmpegPath)
	}
}

func TestFindFFmpegPathOnlyUsesProjectFiles(t *testing.T) {
	t.Setenv("JAVBOSS_BUILD_MODE", "development")
	t.Setenv("JAVBOSS_CONTAINER", "")
	t.Setenv("JAVBOSS_DOCKER", "")
	for _, source := range []string{"none", "bundled", "downloaded"} {
		t.Run(source, func(t *testing.T) {
			baseDir := t.TempDir()
			t.Chdir(baseDir)
			binName := filepath.Base(FFmpegToolRelativePath())
			systemDir := t.TempDir()
			systemPath := filepath.Join(systemDir, binName)
			if err := os.WriteFile(systemPath, []byte("system ffmpeg"), 0o755); err != nil {
				t.Fatal(err)
			}
			t.Setenv("PATH", systemDir)
			t.Setenv("FFMPEG_PATH", systemPath)

			want := ""
			if source == "bundled" {
				want = filepath.Join(baseDir, "internal", "bin", binName)
			} else if source == "downloaded" {
				want = filepath.Join(baseDir, FFmpegToolRelativePath())
			}
			if want != "" {
				if err := os.MkdirAll(filepath.Dir(want), 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(want, []byte("project ffmpeg"), 0o755); err != nil {
					t.Fatal(err)
				}
			}
			if source == "bundled" && runtime.GOOS != "darwin" {
				want = ""
			}
			got, err := findFFmpegPath()
			if want == "" {
				if err == nil || got != "" {
					t.Fatalf("unmanaged FFmpeg was accepted: %q, %v", got, err)
				}
			} else if err != nil || got != want {
				t.Fatalf("findFFmpegPath() = %q, %v; want %q", got, err, want)
			}
		})
	}
}

func TestFindFFprobePathIgnoresEnvironmentAndSystemPath(t *testing.T) {
	t.Setenv("JAVBOSS_BUILD_MODE", "development")
	baseDir := t.TempDir()
	t.Chdir(baseDir)
	t.Setenv("JAVBOSS_CONTAINER", "")
	t.Setenv("JAVBOSS_DOCKER", "")
	binName := "ffprobe" + filepath.Ext(FFmpegToolRelativePath())
	systemDir := t.TempDir()
	systemPath := filepath.Join(systemDir, binName)
	if err := os.WriteFile(systemPath, []byte("system ffprobe"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("FFPROBE_PATH", systemPath)
	t.Setenv("PATH", systemDir)
	if got, err := findFFprobePath(); err == nil || got != "" {
		t.Fatalf("environment/system FFprobe was accepted: %q, %v", got, err)
	}

	bundledPath := filepath.Join(baseDir, "internal", "bin", binName)
	if err := os.MkdirAll(filepath.Dir(bundledPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(bundledPath, []byte("bundled ffprobe"), 0o755); err != nil {
		t.Fatal(err)
	}
	if got, err := findFFprobePath(); err != nil || got != bundledPath {
		t.Fatalf("findFFprobePath() = %q, %v; want %q", got, err, bundledPath)
	}
}

func TestReleaseFFBinaryLookupOnlyUsesExecutableDirectory(t *testing.T) {
	t.Setenv("JAVBOSS_BUILD_MODE", "release")
	t.Setenv("JAVBOSS_CONTAINER", "")
	t.Setenv("JAVBOSS_DOCKER", "")
	t.Chdir(t.TempDir())
	execPath, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	execDir := filepath.Dir(execPath)
	for _, name := range []string{"ffmpeg", "ffprobe"} {
		for _, installed := range []bool{true, false} {
			t.Run(fmt.Sprintf("%s/installed=%t", name, installed), func(t *testing.T) {
				binName := name + filepath.Ext(FFmpegToolRelativePath())
				want := filepath.Join(execDir, "internal", "bin", binName)
				if name == "ffmpeg" {
					want = filepath.Join(execDir, FFmpegToolRelativePath())
				}
				calls := 0
				lookup := func(candidate string) (string, error) {
					calls++
					rel, err := filepath.Rel(execDir, candidate)
					if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || filepath.IsAbs(rel) {
						t.Errorf("release looked outside executable directory: %q", candidate)
						// A working-directory or system binary would be available.
						return candidate, nil
					}
					if installed && candidate == want {
						return candidate, nil
					}
					return "", os.ErrNotExist
				}
				got, err := findFFBinaryPathWithLookup(name, lookup)
				if calls == 0 {
					t.Fatal("executable directory was not checked")
				}
				if installed {
					if err != nil || got != want {
						t.Fatalf("got %q, %v; want %q", got, err, want)
					}
				} else if err == nil || got != "" {
					t.Fatalf("missing release binary must fail without fallback: %q, %v", got, err)
				}
			})
		}
	}
}

func TestDockerFFBinaryLookupOnlyUsesFixedImagePath(t *testing.T) {
	t.Setenv("JAVBOSS_BUILD_MODE", "release")
	t.Setenv("JAVBOSS_CONTAINER", "1")
	t.Setenv("JAVBOSS_DOCKER", "")
	t.Chdir(t.TempDir())
	t.Setenv("FFMPEG_PATH", "/ignored/ffmpeg")
	t.Setenv("FFPROBE_PATH", "/ignored/ffprobe")
	for _, name := range []string{"ffmpeg", "ffprobe"} {
		for _, installed := range []bool{true, false} {
			t.Run(fmt.Sprintf("%s/installed=%t", name, installed), func(t *testing.T) {
				want := "/app/internal/bin/" + name
				var calls []string
				lookup := func(candidate string) (string, error) {
					calls = append(calls, candidate)
					if candidate == want && !installed {
						return "", os.ErrNotExist
					}
					// Other paths would succeed, exposing any unwanted fallback.
					return candidate, nil
				}
				got, err := findFFBinaryPathWithLookup(name, lookup)
				if len(calls) != 1 || calls[0] != want {
					t.Fatalf("lookup paths = %v, want only %s", calls, want)
				}
				if installed {
					if err != nil || got != want {
						t.Fatalf("got %q, %v; want %q", got, err, want)
					}
				} else if err == nil || got != "" {
					t.Fatalf("missing image binary must fail without fallback: %q, %v", got, err)
				}
			})
		}
	}
}

func TestFFBinaryCandidatesForBasePlatformOrder(t *testing.T) {
	baseDir := filepath.Join("project", "root")
	binName := "ffmpeg"
	toolPath := filepath.Join("data", "tools", "platform", binName)
	bundledPath := filepath.Join(baseDir, "internal", "bin", binName)
	downloadedPath := filepath.Join(baseDir, toolPath)

	tests := []struct {
		name string
		goos string
		want []string
	}{
		{name: "macOS prioritizes bundled FFmpeg", goos: "darwin", want: []string{bundledPath, downloadedPath}},
		{name: "Windows only uses tool downloads", goos: "windows", want: []string{downloadedPath}},
		{name: "Linux only uses tool downloads", goos: "linux", want: []string{downloadedPath}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ffBinaryCandidatesForBase(baseDir, "ffmpeg", binName, tt.goos, toolPath)
			if len(got) != len(tt.want) {
				t.Fatalf("candidate count = %d, want %d", len(got), len(tt.want))
			}
			for index := range tt.want {
				if got[index] != tt.want[index] {
					t.Fatalf("candidate[%d] = %q, want %q", index, got[index], tt.want[index])
				}
			}
		})
	}
}
