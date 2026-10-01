package util

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func fakeBrowserTranscoder() *BrowserTranscoder {
	return &BrowserTranscoder{
		binary:     "ffmpeg",
		candidates: []browserVideoEncoder{{name: "h264_nvenc"}, {name: "h264_qsv"}},
		detect: func(context.Context, string) (map[string]bool, error) {
			return map[string]bool{"h264_nvenc": true, "h264_qsv": true, "libx264": true}, nil
		},
		probe: func(context.Context, string, browserVideoEncoder) error { return nil },
		encode: func(_ context.Context, _, _, output string, e browserVideoEncoder, _ *VideoMetadata, progress func(float64, string)) error {
			file, err := os.OpenFile(output, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
			if err != nil {
				return err
			}
			_, writeErr := file.WriteString(e.name)
			closeErr := file.Close()
			if progress != nil {
				progress(1, "2x")
			}
			return errors.Join(writeErr, closeErr)
		},
	}
}

func TestBrowserTranscoderRequiresWorkingHardwareAndCachesSelection(t *testing.T) {
	runner := fakeBrowserTranscoder()
	var probes, encoders []string
	detections := 0
	detect := runner.detect
	runner.detect = func(ctx context.Context, binary string) (map[string]bool, error) {
		detections++
		return detect(ctx, binary)
	}
	runner.probe = func(_ context.Context, _ string, e browserVideoEncoder) error {
		probes = append(probes, e.name)
		if e.name == "h264_nvenc" {
			return errors.New("driver unavailable")
		}
		return nil
	}
	for _, name := range []string{"first.mp4", "second.mp4"} {
		err := runner.Transcode(t.Context(), "source.mkv", filepath.Join(t.TempDir(), name), BrowserTranscodeOptions{EncoderChanged: func(status BrowserEncoderStatus) {
			if !status.Hardware || status.FallbackReason != "" {
				t.Fatalf("unexpected status: %+v", status)
			}
			encoders = append(encoders, status.Name)
		}})
		if err != nil {
			t.Fatal(err)
		}
	}
	if detections != 1 || !reflect.DeepEqual(probes, []string{"h264_nvenc", "h264_qsv"}) || !reflect.DeepEqual(encoders, []string{"h264_qsv", "h264_qsv"}) {
		t.Fatalf("selection repeated or unusable GPU accepted: %d %v %v", detections, probes, encoders)
	}
}

func TestBrowserTranscoderFallsBackWhenNoHardwareWorks(t *testing.T) {
	runner := fakeBrowserTranscoder()
	runner.probe = func(context.Context, string, browserVideoEncoder) error { return errors.New("hardware unavailable") }
	output := filepath.Join(t.TempDir(), "output.mp4")
	err := runner.Transcode(t.Context(), "source.mkv", output, BrowserTranscodeOptions{EncoderChanged: func(status BrowserEncoderStatus) {
		if status.Hardware || status.Name != "libx264" || status.FallbackReason != "unavailable" || !strings.Contains(status.Detail, "hardware unavailable") {
			t.Fatalf("fallback not reported: %+v", status)
		}
	}})
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(output)
	if err != nil || string(data) != "libx264" {
		t.Fatalf("CPU output: %q %v", data, err)
	}
}

func TestBrowserTranscoderRetriesHardwareEncodingAndValidationFailures(t *testing.T) {
	for _, failValidation := range []bool{false, true} {
		t.Run(map[bool]string{false: "encoding failure", true: "validation failure"}[failValidation], func(t *testing.T) {
			runner := fakeBrowserTranscoder()
			encode := runner.encode
			var attempts []string
			runner.encode = func(ctx context.Context, binary, source, output string, e browserVideoEncoder, meta *VideoMetadata, progress func(float64, string)) error {
				attempts = append(attempts, e.name)
				if err := encode(ctx, binary, source, output, e, meta, progress); err != nil {
					return err
				}
				if e.name != "libx264" && !failValidation {
					return errors.New("GPU session failed")
				}
				return nil
			}
			root := t.TempDir()
			source := filepath.Join(root, "source.mkv")
			if err := os.WriteFile(source, []byte("source bytes"), 0600); err != nil {
				t.Fatal(err)
			}
			var statuses []BrowserEncoderStatus
			validate := func(_ context.Context, output string) error {
				data, err := os.ReadFile(output)
				if err != nil {
					return err
				}
				if string(data) != "libx264" && failValidation {
					return errors.New("GPU output failed validation")
				}
				return nil
			}
			for _, name := range []string{"first.mp4", "second.mp4"} {
				output := filepath.Join(root, name)
				err := runner.Transcode(t.Context(), source, output, BrowserTranscodeOptions{Validate: validate, EncoderChanged: func(status BrowserEncoderStatus) { statuses = append(statuses, status) }})
				if err != nil {
					t.Fatal(err)
				}
				data, err := os.ReadFile(output)
				if err != nil || string(data) != "libx264" {
					t.Fatalf("bad software retry output: %q %v", data, err)
				}
			}
			if !reflect.DeepEqual(attempts, []string{"h264_nvenc", "libx264", "libx264"}) {
				t.Fatalf("hardware failure repeated: %v", attempts)
			}
			if len(statuses) != 3 || statuses[1].Hardware || statuses[1].FallbackReason != "encoding_failed" {
				t.Fatalf("fallback missing: %+v", statuses)
			}
			data, err := os.ReadFile(source)
			if err != nil || string(data) != "source bytes" {
				t.Fatal("source modified during retry")
			}
		})
	}
}

func TestBrowserTranscoderDoesNotRetryCancellationOrOverwriteExistingOutput(t *testing.T) {
	for _, scenario := range []string{"probe cancellation", "encode cancellation", "existing output"} {
		t.Run(scenario, func(t *testing.T) {
			runner := fakeBrowserTranscoder()
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			attempts := 0
			output := filepath.Join(t.TempDir(), "output.mp4")
			if scenario == "probe cancellation" {
				runner.probe = func(context.Context, string, browserVideoEncoder) error { cancel(); return context.Canceled }
			}
			if scenario == "existing output" {
				if err := os.WriteFile(output, []byte("existing"), 0600); err != nil {
					t.Fatal(err)
				}
			}
			runner.encode = func(context.Context, string, string, string, browserVideoEncoder, *VideoMetadata, func(float64, string)) error {
				attempts++
				cancel()
				return context.Canceled
			}
			err := runner.Transcode(ctx, "source.mkv", output, BrowserTranscodeOptions{})
			if err == nil {
				t.Fatal("expected failure")
			}
			if scenario == "encode cancellation" && attempts != 1 {
				t.Fatalf("cancelled encoding retried: %d", attempts)
			}
			if scenario != "encode cancellation" && attempts != 0 {
				t.Fatal("encoding should not have started")
			}
			if scenario == "existing output" {
				data, err := os.ReadFile(output)
				if err != nil || string(data) != "existing" {
					t.Fatal("existing output removed")
				}
			} else if !errors.Is(err, context.Canceled) {
				t.Fatalf("cancellation lost: %v", err)
			}
		})
	}
}

func TestBrowserHardwareArgumentsRespectDeviceAndFrameFormats(t *testing.T) {
	for _, goos := range []string{"linux", "windows", "darwin"} {
		for _, e := range browserHardwareEncoders(goos, []string{"/dev/dri/renderD129"}) {
			t.Run(goos+"/"+e.name, func(t *testing.T) {
				args := transcodeBrowserMP4Args("source.mkv", "output.mp4", e, &VideoMetadata{Width: 3840, Height: 2160, FPS: 30})
				joined := strings.Join(args, " ")
				if strings.Contains(joined, "-crf") {
					t.Fatal("software CRF used for hardware encoding")
				}
				if !strings.Contains(joined, "-c:v "+e.name) || !strings.Contains(joined, "-c:a aac") || !strings.Contains(joined, "-n ") {
					t.Fatalf("missing safety or codec arguments: %v", args)
				}
				if e.name == "h264_vaapi" && (!strings.Contains(joined, "format=nv12,hwupload") || strings.Contains(joined, "-pix_fmt yuv420p")) {
					t.Fatalf("VAAPI received CPU frames: %v", args)
				}
				if len(e.deviceArgs) > 0 && strings.Index(joined, e.deviceArgs[0]) > strings.Index(joined, "-i source.mkv") {
					t.Fatal("device initialized after input")
				}
				if e.name == "h264_videotoolbox" && !strings.Contains(joined, "-allow_sw 0") {
					t.Fatal("VideoToolbox can silently use software")
				}
				if e.adaptiveBitrate && !strings.Contains(joined, "-b:v 37324800") {
					t.Fatalf("4K bitrate did not scale: %v", args)
				}
			})
		}
	}
}
