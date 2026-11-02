package directory

import (
	"bytes"
	"encoding/base64"
	"errors"
	"image"
	"image/color"
	"image/draw"
	"image/jpeg"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"testing"

	"javboss/internal/models"
)

func TestPortraitCropMatchesCard(t *testing.T) {
	for _, tc := range []struct {
		name     string
		size     image.Rectangle
		position float64
		want     image.Rectangle
	}{
		{"right", image.Rect(0, 0, 800, 538), 1, image.Rect(424, 0, 800, 538)},
		{"left", image.Rect(0, 0, 800, 538), 0, image.Rect(0, 0, 376, 538)},
		{"center", image.Rect(0, 0, 800, 538), 0.5, image.Rect(212, 0, 588, 538)},
		{"tall centered vertically", image.Rect(0, 0, 200, 800), 1, image.Rect(0, 257, 200, 543)},
		{"offset bounds", image.Rect(10, 20, 810, 558), 1, image.Rect(434, 20, 810, 558)},
		{"invalid defaults right", image.Rect(0, 0, 800, 538), math.NaN(), image.Rect(424, 0, 800, 538)},
		{"clamp left", image.Rect(0, 0, 800, 538), -1, image.Rect(0, 0, 376, 538)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := portraitCropBounds(tc.size, tc.position); got != tc.want {
				t.Fatalf("crop = %v, want %v", got, tc.want)
			}
		})
	}
}

func writePortraitTestCover(t *testing.T, dir, extension string) string {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 800, 538))
	for i, c := range []color.RGBA{{R: 255, A: 255}, {G: 255, A: 255}, {B: 255, A: 255}} {
		draw.Draw(img, image.Rect(i*800/3, 0, (i+1)*800/3, 538), &image.Uniform{C: c}, image.Point{}, draw.Src)
	}
	var data bytes.Buffer
	var err error
	if extension == ".png" {
		encoder := png.Encoder{CompressionLevel: png.NoCompression}
		err = encoder.Encode(&data, img)
	} else {
		err = jpeg.Encode(&data, img, &jpeg.Options{Quality: 95})
	}
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "ipx-001"+extension)
	writeTestFile(t, path, data.Bytes())
	return path
}

func TestDirectoryProcessingAlwaysExportsPortrait(t *testing.T) {
	coverDir := t.TempDir()
	writePortraitTestCover(t, coverDir, ".png")
	for _, tc := range []struct {
		name     string
		mode     string
		enabled  bool
		position float64
		channel  int
	}{
		{"fanart disabled", DirectoryProcessSidecar, false, 1, 2},
		{"right", DirectoryProcessSidecar, true, 1, 2},
		{"left", DirectoryProcessSidecar, true, 0, 0},
		{"center", DirectoryProcessSidecar, true, 0.5, 1},
		{"organize and export", DirectoryProcessOrganizeWithSidecar, true, 1, 2},
		{"organize and export without fanart", DirectoryProcessOrganizeWithSidecar, false, 1, 2},
		{"organize ignores option", DirectoryProcessOrganize, true, 1, 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			writeTestFile(t, filepath.Join(root, "video.mp4"), []byte("video"))
			// Keeping an existing full cover must not skip the new portrait export.
			writeTestFile(t, filepath.Join(root, "video-fanart.png"), []byte("existing cover"))
			item := models.Jav{Code: "IPX-001", CoverCropPosition: tc.position, Videos: []models.Video{{Path: "video.mp4"}}}
			summary := &DirectoryProcessSummary{}
			processJavItem(t.Context(), root, &item, tc.mode, DirectoryProcessLayoutCode, coverDir, summary, false, "媒体库", tc.enabled)
			if summary.Failed != 0 {
				t.Fatalf("processing failed: %+v", summary)
			}
			target := root
			if tc.mode != DirectoryProcessSidecar {
				target = filepath.Join(root, "媒体库", "IPX-001")
			}
			fanart, err := os.ReadFile(filepath.Join(target, "video-fanart.png"))
			if err != nil || string(fanart) != "existing cover" {
				t.Fatalf("existing cover changed: %q, %v", fanart, err)
			}
			data, err := os.ReadFile(filepath.Join(target, "video-poster.jpg"))
			if tc.mode == DirectoryProcessOrganize {
				if !errors.Is(err, os.ErrNotExist) {
					t.Fatalf("unexpected portrait export: %v", err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			img, err := jpeg.Decode(bytes.NewReader(data))
			if err != nil {
				t.Fatal(err)
			}
			if img.Bounds() != image.Rect(0, 0, 376, 538) {
				t.Fatalf("unexpected portrait size: %v", img.Bounds())
			}
			r, g, b, _ := img.At(188, 269).RGBA()
			if []uint32{r, g, b}[tc.channel] < 60000 {
				t.Fatalf("wrong crop position: center color %d %d %d", r, g, b)
			}
		})
	}
}

func TestSidecarArtworkNamesAndContent(t *testing.T) {
	for _, extension := range []string{".jpg", ".png"} {
		for _, exportFanart := range []bool{false, true} {
			root, coverDir := t.TempDir(), t.TempDir()
			coverPath := writePortraitTestCover(t, coverDir, extension)
			video := filepath.Join(root, "video.mp4")
			writeTestFile(t, video, []byte("video"))
			if err := writeJavSidecars(video, &models.Jav{Code: "IPX-001", CoverCropPosition: 1}, coverDir, false, exportFanart); err != nil {
				t.Fatal(err)
			}
			original, err := os.ReadFile(coverPath)
			if err != nil {
				t.Fatal(err)
			}
			fanart, err := os.ReadFile(filepath.Join(root, "video-fanart"+extension))
			if exportFanart {
				if err != nil || !bytes.Equal(fanart, original) {
					t.Fatalf("fanart must contain the unchanged full cover: %v", err)
				}
			} else {
				if !errors.Is(err, os.ErrNotExist) {
					t.Fatalf("fanart must not be exported when disabled: %v", err)
				}
			}
			poster, err := os.ReadFile(filepath.Join(root, "video-poster.jpg"))
			if err != nil {
				t.Fatal(err)
			}
			img, err := jpeg.Decode(bytes.NewReader(poster))
			if err != nil || img.Bounds() != image.Rect(0, 0, 376, 538) {
				t.Fatalf("poster must contain the portrait crop: %v", err)
			}
		}
	}
}

func TestPortraitSidecarPreservesExistingFilesAndHandlesBadCover(t *testing.T) {
	for _, managed := range []bool{false, true} {
		root, coverDir := t.TempDir(), t.TempDir()
		writePortraitTestCover(t, coverDir, ".jpg")
		video := filepath.Join(root, "video.mp4")
		portrait := filepath.Join(root, "video-poster.jpg")
		writeTestFile(t, video, []byte("video"))
		writeTestFile(t, portrait, []byte("original portrait"))
		if managed {
			writeTestFile(t, filepath.Join(root, "video.nfo"), []byte("<generator>JavBoss</generator>"))
		}
		if err := writeJavSidecars(video, &models.Jav{Code: "IPX-001", CoverCropPosition: 1}, coverDir, false, false); err != nil {
			t.Fatal(err)
		}
		data, err := os.ReadFile(portrait)
		if err != nil {
			t.Fatal(err)
		}
		if managed {
			if _, err := jpeg.Decode(bytes.NewReader(data)); err != nil {
				t.Fatalf("managed portrait not refreshed: %v", err)
			}
		} else if string(data) != "original portrait" {
			t.Fatal("unmanaged portrait changed")
		}
	}
	root := t.TempDir()
	invalid := filepath.Join(root, "bad.jpg")
	target := filepath.Join(root, "video-poster.jpg")
	writeTestFile(t, invalid, []byte("not an image"))
	writeTestFile(t, target, []byte("keep"))
	if err := writePortraitSidecar(invalid, target, 1); err == nil {
		t.Fatal("invalid cover should fail")
	}
	if data, err := os.ReadFile(target); err != nil || string(data) != "keep" {
		t.Fatalf("failed export changed the target: %q, %v", data, err)
	}
}

func TestPortraitSidecarDecodesWebP(t *testing.T) {
	// An 8x4 solid blue lossless WebP, generated with FFmpeg.
	data, err := base64.StdEncoding.DecodeString("UklGRhwAAABXRUJQVlA4TA8AAAAvB8AAAAcQ0f/+ByKi/wEA")
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	source, target := filepath.Join(root, "cover.webp"), filepath.Join(root, "video-poster.jpg")
	writeTestFile(t, source, data)
	if err := writePortraitSidecar(source, target, 1); err != nil {
		t.Fatal(err)
	}
	data, err = os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	img, err := jpeg.Decode(bytes.NewReader(data))
	if err != nil || img.Bounds() != image.Rect(0, 0, 3, 4) {
		t.Fatalf("WebP portrait export failed: %v", err)
	}
}
