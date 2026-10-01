package util

import (
	"context"
	"errors"
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
)

const browserScaleFilter = "scale=trunc(iw/2)*2:trunc(ih/2)*2"

type browserVideoEncoder struct {
	name            string
	deviceArgs      []string
	options         []string
	filter          string
	pixelFormat     string
	adaptiveBitrate bool
}

func softwareBrowserEncoder() browserVideoEncoder {
	return browserVideoEncoder{name: "libx264", options: []string{"-preset", "medium", "-crf", "20"}, filter: browserScaleFilter, pixelFormat: "yuv420p"}
}

func (e browserVideoEncoder) videoArgs(meta *VideoMetadata) []string {
	args := []string{"-c:v", e.name}
	args = append(args, e.options...)
	if e.adaptiveBitrate {
		// VideoToolbox quality mode is unavailable on some Intel Macs. Scale
		// the bitrate to resolution/frame rate instead of imposing a 1080p cap.
		width, height, fps := 1920, 1080, 30.0
		if meta != nil {
			if meta.Width > 0 && meta.Height > 0 {
				width, height = meta.Width, meta.Height
			}
			if meta.FPS > 0 && !math.IsNaN(meta.FPS) && !math.IsInf(meta.FPS, 0) {
				fps = meta.FPS
			}
		}
		bitrate := int64(math.Max(2e6, math.Min(100e6, float64(width)*float64(height)*fps*0.15)))
		args = append(args, "-b:v", strconv.FormatInt(bitrate, 10))
	}
	args = append(args, "-vf", e.filter)
	if e.pixelFormat != "" {
		args = append(args, "-pix_fmt", e.pixelFormat)
	}
	return args
}

func browserHardwareEncoders(goos string, renderNodes []string) []browserVideoEncoder {
	if goos == "darwin" {
		quality := browserVideoEncoder{name: "h264_videotoolbox", options: []string{"-allow_sw", "0", "-q:v", "65"}, filter: browserScaleFilter, pixelFormat: "yuv420p"}
		bitrate := quality
		bitrate.options = []string{"-allow_sw", "0"}
		bitrate.adaptiveBitrate = true
		return []browserVideoEncoder{quality, bitrate}
	}
	if goos != "linux" && goos != "windows" {
		return nil
	}
	encoders := []browserVideoEncoder{{name: "h264_nvenc", options: []string{"-preset", "medium", "-rc", "vbr", "-cq", "20", "-b:v", "0"}, filter: browserScaleFilter, pixelFormat: "yuv420p"}}
	qsv := browserVideoEncoder{name: "h264_qsv", deviceArgs: []string{"-init_hw_device", "qsv:hw"}, options: []string{"-preset", "medium", "-global_quality", "20", "-look_ahead", "0"}, filter: browserScaleFilter + ",format=nv12", pixelFormat: "nv12"}
	if goos == "windows" {
		encoders = append(encoders, qsv, browserVideoEncoder{name: "h264_amf", options: []string{"-quality", "balanced", "-rc", "cqp", "-qp_i", "20", "-qp_p", "20", "-qp_b", "20"}, filter: browserScaleFilter + ",format=nv12", pixelFormat: "nv12"})
	} else {
		for _, node := range renderNodes {
			candidate := qsv
			candidate.deviceArgs = []string{"-init_hw_device", "qsv:hw,child_device=" + node}
			encoders = append(encoders, candidate)
		}
		for _, node := range renderNodes {
			encoders = append(encoders, browserVideoEncoder{name: "h264_vaapi", deviceArgs: []string{"-vaapi_device", node}, options: []string{"-rc_mode", "CQP", "-qp", "20"}, filter: browserScaleFilter + ",format=nv12,hwupload"})
		}
	}
	return encoders
}

type BrowserEncoderStatus struct {
	Name           string
	Hardware       bool
	FallbackReason string // unavailable or encoding_failed; empty when hardware is active.
	Detail         string
}

type BrowserTranscodeOptions struct {
	Metadata       *VideoMetadata
	Progress       func(float64, string)
	EncoderChanged func(BrowserEncoderStatus)
	Validate       func(context.Context, string) error
}

// BrowserTranscoder is owned by one sequential directory job. Hardware is tested
// once per job, using a real short encode rather than just the compiled codec list.
// Decoding/filtering remain in software for broad input-format compatibility.
type BrowserTranscoder struct {
	binary     string
	candidates []browserVideoEncoder
	encoder    *browserVideoEncoder
	status     BrowserEncoderStatus
	detect     func(context.Context, string) (map[string]bool, error)
	probe      func(context.Context, string, browserVideoEncoder) error
	encode     func(context.Context, string, string, string, browserVideoEncoder, *VideoMetadata, func(float64, string)) error
}

func NewBrowserTranscoder(binary string) *BrowserTranscoder {
	nodes, _ := filepath.Glob("/dev/dri/renderD*")
	return &BrowserTranscoder{binary: binary, candidates: browserHardwareEncoders(runtime.GOOS, nodes), detect: availableBrowserEncoders, probe: probeBrowserEncoder, encode: transcodeBrowserMP4WithEncoder}
}

func availableBrowserEncoders(ctx context.Context, binary string) (map[string]bool, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	output, err := exec.CommandContext(ctx, binary, "-hide_banner", "-encoders").Output()
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	if err != nil {
		return nil, fmt.Errorf("list ffmpeg encoders: %w", err)
	}
	result := map[string]bool{}
	for _, line := range strings.Split(string(output), "\n") {
		fields := strings.Fields(line)
		if len(fields) >= 2 && len(fields[0]) == 6 && strings.HasPrefix(fields[0], "V") {
			result[fields[1]] = true
		}
	}
	return result, nil
}

func probeBrowserEncoder(ctx context.Context, binary string, encoder browserVideoEncoder) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	args := []string{"-hide_banner", "-loglevel", "error", "-nostdin", "-xerror"}
	args = append(args, encoder.deviceArgs...)
	args = append(args, "-f", "lavfi", "-i", "color=c=black:s=320x240:r=30")
	args = append(args, encoder.videoArgs(&VideoMetadata{Width: 320, Height: 240, FPS: 30})...)
	args = append(args, "-frames:v", "3", "-an", "-f", "null", "-")
	cmd := exec.CommandContext(ctx, binary, args...)
	var stderr transcodeLog
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return fmt.Errorf("%s: %w: %s", encoder.name, err, strings.TrimSpace(stderr.text))
	}
	return nil
}

func (t *BrowserTranscoder) selectEncoder(ctx context.Context) error {
	available, err := t.detect(ctx, t.binary)
	if err != nil {
		return err
	}
	var failures []string
	for _, candidate := range t.candidates {
		if err := ctx.Err(); err != nil {
			return err
		}
		if !available[candidate.name] {
			continue
		}
		if err := t.probe(ctx, t.binary, candidate); err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			failures = append(failures, err.Error())
			continue
		}
		t.encoder = &candidate
		t.status = BrowserEncoderStatus{Name: candidate.name, Hardware: true}
		return nil
	}
	if !available["libx264"] {
		return errors.New("no usable hardware encoder and ffmpeg has no libx264 encoder")
	}
	software := softwareBrowserEncoder()
	t.encoder = &software
	t.status = BrowserEncoderStatus{Name: software.name, FallbackReason: "unavailable", Detail: strings.Join(failures, "\n")}
	return nil
}

func (t *BrowserTranscoder) Transcode(ctx context.Context, source, output string, options BrowserTranscodeOptions) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	// Never delete a caller's existing file while cleaning up a failed GPU attempt.
	if _, err := os.Lstat(output); !errors.Is(err, os.ErrNotExist) {
		if err == nil {
			return fmt.Errorf("output already exists: %s", output)
		}
		return err
	}
	if t.encoder == nil {
		if err := t.selectEncoder(ctx); err != nil {
			return err
		}
	}
	run := func() error {
		if options.EncoderChanged != nil {
			options.EncoderChanged(t.status)
		}
		err := t.encode(ctx, t.binary, source, output, *t.encoder, options.Metadata, options.Progress)
		if err == nil && options.Validate != nil {
			err = options.Validate(ctx, output)
		}
		return err
	}
	err := run()
	if err == nil {
		return nil
	}
	if ctx.Err() != nil {
		return ctx.Err()
	}
	if !t.status.Hardware {
		return err
	}
	// Driver/session failures and invalid GPU outputs get one complete software
	// retry. Keep software for the remaining files instead of repeating failures.
	if removeErr := os.Remove(output); removeErr != nil && !errors.Is(removeErr, os.ErrNotExist) {
		return errors.Join(err, removeErr)
	}
	software := softwareBrowserEncoder()
	t.encoder = &software
	t.status = BrowserEncoderStatus{Name: software.name, FallbackReason: "encoding_failed", Detail: err.Error()}
	if retryErr := run(); retryErr != nil {
		return fmt.Errorf("hardware attempt failed (%v); software retry: %w", err, retryErr)
	}
	return nil
}
