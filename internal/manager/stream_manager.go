package manager

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"javboss/internal/common/logging"
	"javboss/internal/util"
)

const (
	MimeHLS    = "application/vnd.apple.mpegurl"
	MimeMpegTS = "video/MP2T"

	segmentLength = 2

	maxSegmentWait   = 15 * time.Second
	monitorInterval  = 200 * time.Millisecond
	maxSegmentGap    = 5
	maxSegmentBuffer = 15
	maxIdleTime      = 30 * time.Second

	resolutionParamKey = "resolution"
	apiKeyParamKey     = "apikey"
	locationIDParamKey = "location_id"
)

var ErrInvalidSegment = errors.New("invalid segment")

type StreamManager struct {
	cacheDir string

	context    context.Context
	cancelFunc context.CancelFunc

	runningStreams map[string]*runningStream
	streamsMutex   sync.Mutex
}

type StreamOptions struct {
	StreamType *StreamType
	SourcePath string
	Duration   float64
	Resolution string
	Key        string
	Segment    string
}

type StreamType struct {
	Name          string
	SegmentType   *SegmentType
	ServeManifest func(sm *StreamManager, w http.ResponseWriter, r *http.Request, sourcePath string, durationHint float64, resolution string)
	Args          func(segment int, videoFilter string, videoOnly bool, outputDir string) []string
}

type SegmentType struct {
	Format       string
	MimeType     string
	MakeFilename func(segment int) string
	ParseSegment func(str string) (int, error)
}

type streamVideoFile struct {
	Path       string
	Width      int
	Height     int
	Duration   float64
	AudioCodec string
}

type transcodeProcess struct {
	cmd         *exec.Cmd
	context     context.Context
	cancel      context.CancelFunc
	cancelled   bool
	outputDir   string
	segmentType *SegmentType
	segment     int
}

type waitingSegment struct {
	segmentType *SegmentType
	idx         int
	file        string
	path        string
	accessed    time.Time
	available   chan error
	done        atomic.Bool
}

type runningStream struct {
	dir              string
	streamType       *StreamType
	vf               *streamVideoFile
	maxTranscodeSize int
	outputDir        string

	waitingSegments []*waitingSegment
	tp              *transcodeProcess
	lastAccessed    time.Time
	lastSegment     int
}

var (
	SegmentTypeTS = &SegmentType{
		Format:   "%d.ts",
		MimeType: MimeMpegTS,
		MakeFilename: func(segment int) string {
			return fmt.Sprintf("%d.ts", segment)
		},
		ParseSegment: func(str string) (int, error) {
			str = strings.TrimSuffix(strings.TrimSpace(str), ".ts")
			segment, err := strconv.Atoi(str)
			if err != nil || segment < 0 {
				return 0, ErrInvalidSegment
			}
			return segment, nil
		},
	}

	StreamTypeHLS = &StreamType{
		Name:        "hls",
		SegmentType: SegmentTypeTS,
		ServeManifest: func(sm *StreamManager, w http.ResponseWriter, r *http.Request, sourcePath string, durationHint float64, resolution string) {
			serveHLSManifest(sm, w, r, sourcePath, durationHint, resolution)
		},
		Args: func(segment int, videoFilter string, videoOnly bool, outputDir string) []string {
			args := []string{
				"-c:v", "libx264",
				"-pix_fmt", "yuv420p",
				"-preset", "veryfast",
				"-crf", "25",
				"-sc_threshold", "0",
				"-flags", "+cgop",
				"-force_key_frames", fmt.Sprintf("expr:gte(t,n_forced*%d)", segmentLength),
			}
			if videoFilter != "" {
				args = append(args, "-vf", videoFilter)
			}
			if videoOnly {
				args = append(args, "-an")
			} else {
				args = append(args, "-c:a", "aac", "-ac", "2")
			}
			args = append(args,
				"-sn",
				"-copyts",
				"-avoid_negative_ts", "disabled",
				"-f", "hls",
				"-start_number", fmt.Sprint(segment),
				"-hls_time", fmt.Sprint(segmentLength),
				"-hls_flags", "split_by_time",
				"-hls_segment_type", "mpegts",
				"-hls_playlist_type", "vod",
				"-hls_segment_filename", filepath.Join(outputDir, ".%d.ts"),
				filepath.Join(outputDir, "manifest.m3u8"),
			)
			return args
		},
	}
)

var streamingResolutionMax = map[string]int{
	"LOW":         240,
	"STANDARD":    480,
	"STANDARD_HD": 720,
	"FULL_HD":     1080,
	"FOUR_K":      1920,
	"ORIGINAL":    0,
	"":            0,
}

func NewStreamManager(cacheDir string) *StreamManager {
	ctx, cancel := context.WithCancel(context.Background())
	return &StreamManager{
		cacheDir:       cacheDir,
		context:        ctx,
		cancelFunc:     cancel,
		runningStreams: make(map[string]*runningStream),
	}
}

func (sm *StreamManager) Start(parent context.Context) {
	go func() {
		ticker := time.NewTicker(monitorInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				sm.monitorStreams()
			case <-parent.Done():
				sm.Stop()
				return
			case <-sm.context.Done():
				return
			}
		}
	}()
}

func (sm *StreamManager) Stop() {
	sm.cancelFunc()
	sm.stopAndRemoveAll()
}

func (sm *StreamManager) ServeManifest(w http.ResponseWriter, r *http.Request, sourcePath string, durationHint float64, resolution string) {
	StreamTypeHLS.ServeManifest(sm, w, r, sourcePath, durationHint, resolution)
}

func (sm *StreamManager) ServeSegment(w http.ResponseWriter, r *http.Request, options StreamOptions) {
	if sm.cacheDir == "" {
		http.Error(w, "cannot live transcode files because cache dir is unset", http.StatusServiceUnavailable)
		return
	}
	if options.StreamType == nil || options.StreamType.SegmentType == nil {
		http.Error(w, "invalid stream type", http.StatusBadRequest)
		return
	}
	if options.Key == "" {
		http.Error(w, "invalid key", http.StatusBadRequest)
		return
	}

	segment, err := options.StreamType.SegmentType.ParseSegment(options.Segment)
	if err != nil {
		http.Error(w, "invalid segment", http.StatusBadRequest)
		return
	}

	if segment > lastSegmentForDuration(options.Duration) {
		http.Error(w, "invalid segment", http.StatusBadRequest)
		return
	}

	maxTranscodeSize := maxResolution(options.Resolution)
	dir := options.StreamType.FileDir(options.Key, maxTranscodeSize)
	outputDir := filepath.Join(sm.cacheDir, dir)
	name := options.StreamType.SegmentType.MakeFilename(segment)
	file := filepath.Join(dir, name)

	sm.streamsMutex.Lock()

	stream := sm.runningStreams[dir]
	if stream == nil {
		vf, err := loadStreamVideoFile(options.SourcePath, options.Duration)
		if err != nil {
			sm.streamsMutex.Unlock()
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		stream = &runningStream{
			dir:              dir,
			streamType:       options.StreamType,
			vf:               vf,
			maxTranscodeSize: maxTranscodeSize,
			outputDir:        outputDir,
			waitingSegments:  make([]*waitingSegment, 0, 10),
		}
		sm.runningStreams[dir] = stream
	}

	now := time.Now()
	stream.lastAccessed = now
	stream.lastSegment = segment

	waiting := &waitingSegment{
		segmentType: options.StreamType.SegmentType,
		idx:         segment,
		file:        file,
		path:        filepath.Join(sm.cacheDir, file),
		accessed:    now,
		available:   make(chan error, 1),
	}
	stream.waitingSegments = append(stream.waitingSegments, waiting)

	sm.streamsMutex.Unlock()

	sm.serveWaitingSegment(w, r, waiting)
}

func (sm *StreamManager) serveWaitingSegment(w http.ResponseWriter, r *http.Request, segment *waitingSegment) {
	select {
	case <-r.Context().Done():
	case err := <-segment.available:
		if err == nil {
			w.Header().Set("Content-Type", segment.segmentType.MimeType)
			http.ServeFile(w, r, segment.path)
		} else if !errors.Is(err, context.Canceled) {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	}
	segment.done.Store(true)
}

func (sm *StreamManager) startTranscode(stream *runningStream, segment int, done chan<- error) {
	if segment < 0 {
		segment = 0
	}

	if err := os.MkdirAll(stream.outputDir, os.ModePerm); err != nil {
		done <- err
		return
	}

	ffmpegPath, err := util.ResolveFFmpegPath()
	if err != nil {
		done <- err
		return
	}

	lockCtx, cancel := context.WithCancel(sm.context)
	args := stream.makeStreamArgs(segment)
	cmd := exec.CommandContext(lockCtx, ffmpegPath, args...)

	stderr, err := cmd.StderrPipe()
	if err != nil {
		cancel()
		done <- err
		return
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		cancel()
		done <- err
		return
	}

	if err := cmd.Start(); err != nil {
		cancel()
		done <- fmt.Errorf("error starting transcode process: %w", err)
		return
	}

	tp := &transcodeProcess{
		cmd:         cmd,
		context:     lockCtx,
		cancel:      cancel,
		outputDir:   stream.outputDir,
		segmentType: stream.streamType.SegmentType,
		segment:     segment,
	}
	stream.tp = tp

	go func() {
		errStr, _ := io.ReadAll(stderr)
		outStr, _ := io.ReadAll(stdout)
		errCmd := cmd.Wait()

		var runErr error
		if !tp.cancelled {
			msg := string(errStr)
			if msg == "" {
				msg = string(outStr)
			}
			if msg != "" {
				runErr = errors.New(msg)
			} else {
				runErr = errCmd
			}
			if runErr != nil {
				runErr = fmt.Errorf("ffmpeg error when running command <%s>: %w", strings.Join(cmd.Args, " "), runErr)
				logging.Error("transcode error: %v", runErr)
			}
		}

		sm.streamsMutex.Lock()
		tp.cancel()
		tp.checkSegments()
		if stream.tp == tp {
			stream.tp = nil
		}
		sm.streamsMutex.Unlock()

		if runErr != nil {
			done <- runErr
		}
	}()
}

func (sm *StreamManager) stopTranscode(stream *runningStream) {
	tp := stream.tp
	if tp != nil {
		tp.cancel()
		tp.cancelled = true
	}
}

func (sm *StreamManager) checkTranscode(stream *runningStream, now time.Time) {
	if len(stream.waitingSegments) == 0 && stream.lastAccessed.Add(maxIdleTime).Before(now) {
		sm.stopTranscode(stream)
		sm.removeTranscodeFiles(stream)
		delete(sm.runningStreams, stream.dir)
		return
	}

	if stream.tp != nil {
		segmentType := stream.streamType.SegmentType
		segment := stream.lastSegment
		for i := segment; i < segment+maxSegmentBuffer; i++ {
			if !segmentExists(filepath.Join(stream.outputDir, segmentType.MakeFilename(i))) {
				return
			}
		}
		sm.stopTranscode(stream)
	}
}

func (sm *StreamManager) ensureTranscode(stream *runningStream, segment *waitingSegment) bool {
	segmentIdx := segment.idx
	tp := stream.tp
	if tp == nil {
		sm.startTranscode(stream, segmentIdx, segment.available)
		return true
	}
	if segmentIdx < tp.segment || tp.segment+maxSegmentGap < segmentIdx {
		sm.stopTranscode(stream)
		return true
	}
	return false
}

func (sm *StreamManager) monitorStreams() {
	sm.streamsMutex.Lock()
	defer sm.streamsMutex.Unlock()

	now := time.Now()
	for _, stream := range sm.runningStreams {
		if stream.tp != nil {
			stream.tp.checkSegments()
		}

		transcodeStarted := false
		temp := stream.waitingSegments[:0]
		for _, segment := range stream.waitingSegments {
			remove := false
			if segment.done.Load() || segment.checkAvailable(now) {
				remove = true
			} else if !transcodeStarted {
				transcodeStarted = sm.ensureTranscode(stream, segment)
			}
			if !remove {
				temp = append(temp, segment)
			}
		}
		stream.waitingSegments = temp

		if !transcodeStarted {
			sm.checkTranscode(stream, now)
		}
	}
}

func (sm *StreamManager) removeTranscodeFiles(stream *runningStream) {
	if err := os.RemoveAll(stream.outputDir); err != nil {
		logging.Error("remove segment directory failed %s: %v", stream.outputDir, err)
	}
}

func (sm *StreamManager) stopAndRemoveAll() {
	sm.streamsMutex.Lock()
	defer sm.streamsMutex.Unlock()

	for _, stream := range sm.runningStreams {
		for _, segment := range stream.waitingSegments {
			if len(segment.available) == 0 {
				segment.available <- context.Canceled
			}
		}
		sm.stopTranscode(stream)
		sm.removeTranscodeFiles(stream)
	}

	sm.runningStreams = nil
}

func serveHLSManifest(sm *StreamManager, w http.ResponseWriter, r *http.Request, sourcePath string, durationHint float64, resolution string) {
	if sm.cacheDir == "" {
		http.Error(w, "cannot live transcode with HLS because cache dir is unset", http.StatusServiceUnavailable)
		return
	}

	vf, err := loadStreamVideoFile(sourcePath, durationHint)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	videoWidth := vf.Width
	videoHeight := vf.Height

	urlQuery := url.Values{}
	for _, key := range []string{apiKeyParamKey, locationIDParamKey} {
		if value := strings.TrimSpace(r.URL.Query().Get(key)); value != "" {
			urlQuery.Set(key, value)
		}
	}

	maxTranscodeSize := maxResolution(resolution)
	if resolution != "" {
		urlQuery.Set(resolutionParamKey, resolution)
	}
	if maxTranscodeSize != 0 && videoWidth > 0 && videoHeight > 0 {
		videoSize := videoHeight
		if videoWidth < videoSize {
			videoSize = videoWidth
		}
		if maxTranscodeSize < videoSize {
			scaleFactor := float64(maxTranscodeSize) / float64(videoSize)
			videoWidth = int(float64(videoWidth) * scaleFactor)
			videoHeight = int(float64(videoHeight) * scaleFactor)
		}
	}

	urlQueryString := ""
	if len(urlQuery) > 0 {
		urlQueryString = "?" + urlQuery.Encode()
	}

	prefix := r.Header.Get("X-Forwarded-Prefix")
	baseURL := *r.URL
	baseURL.RawQuery = ""
	basePath := prefix + baseURL.String()

	var buf bytes.Buffer
	fmt.Fprint(&buf, "#EXTM3U\n")
	fmt.Fprint(&buf, "#EXT-X-VERSION:3\n")
	fmt.Fprint(&buf, "#EXT-X-MEDIA-SEQUENCE:0\n")
	fmt.Fprintf(&buf, "#EXT-X-TARGETDURATION:%d\n", segmentLength)
	fmt.Fprint(&buf, "#EXT-X-PLAYLIST-TYPE:VOD\n")

	leftover := vf.Duration
	if leftover <= 0 {
		leftover = float64(segmentLength)
	}
	segment := 0
	for leftover > 0 {
		thisLength := float64(segmentLength)
		if leftover < thisLength {
			thisLength = leftover
		}
		fmt.Fprintf(&buf, "#EXTINF:%f,\n", thisLength)
		fmt.Fprintf(&buf, "%s/%d.ts%s\n", basePath, segment, urlQueryString)
		leftover -= thisLength
		segment++
	}
	fmt.Fprint(&buf, "#EXT-X-ENDLIST\n")

	w.Header().Set("Content-Type", MimeHLS)
	_, _ = w.Write(buf.Bytes())
}

func (t StreamType) FileDir(key string, maxTranscodeSize int) string {
	if maxTranscodeSize == 0 {
		return fmt.Sprintf("%s_%s", key, t.Name)
	}
	return fmt.Sprintf("%s_%s_%d", key, t.Name, maxTranscodeSize)
}

func (s *runningStream) makeStreamArgs(segment int) []string {
	args := []string{"-hide_banner", "-loglevel", "error"}
	if segment > 0 {
		args = append(args, "-ss", strconv.Itoa(segment*segmentLength))
	}
	args = append(args, "-i", s.vf.Path)

	videoOnly := strings.TrimSpace(s.vf.AudioCodec) == ""
	videoFilter := scaleFilter(s.vf, s.maxTranscodeSize)
	args = append(args, s.streamType.Args(segment, videoFilter, videoOnly, s.outputDir)...)
	return args
}

func (tp *transcodeProcess) checkSegments() {
	doSegment := func(filename string) {
		if filename == "" {
			return
		}
		oldPath := filepath.Join(tp.outputDir, filename)
		newPath := filepath.Join(tp.outputDir, filename[1:])
		if !segmentExists(newPath) {
			_ = os.Rename(oldPath, newPath)
		} else {
			_ = os.Remove(oldPath)
		}
	}

	processState := tp.cmd.ProcessState
	var lastFilename string
	for i := tp.segment; ; i++ {
		filename := fmt.Sprintf("."+tp.segmentType.Format, i)
		if segmentExists(filepath.Join(tp.outputDir, filename)) {
			doSegment(lastFilename)
		} else {
			if processState != nil {
				if processState.Success() {
					doSegment(lastFilename)
				} else if lastFilename != "" {
					_ = os.Remove(filepath.Join(tp.outputDir, lastFilename))
				}
			}
			break
		}
		lastFilename = filename
		tp.segment = i
	}
}

func (s *waitingSegment) checkAvailable(now time.Time) bool {
	if segmentExists(s.path) {
		s.available <- nil
		return true
	}
	if s.accessed.Add(maxSegmentWait).Before(now) {
		s.available <- fmt.Errorf("timed out waiting for segment file %s to be generated", s.file)
		return true
	}
	return false
}

func segmentExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func lastSegmentForDuration(duration float64) int {
	if duration <= 0 {
		return 0
	}
	return int(math.Ceil(duration/segmentLength)) - 1
}

func loadStreamVideoFile(sourcePath string, durationHint float64) (*streamVideoFile, error) {
	meta, err := util.ProbeVideo(sourcePath)
	if err != nil {
		return nil, fmt.Errorf("probe video file: %w", err)
	}
	duration := meta.DurationSeconds
	if duration <= 0 {
		duration = durationHint
	}
	return &streamVideoFile{
		Path:       sourcePath,
		Width:      meta.Width,
		Height:     meta.Height,
		Duration:   duration,
		AudioCodec: meta.AudioCodec,
	}, nil
}

func maxResolution(resolution string) int {
	resolution = strings.ToUpper(strings.TrimSpace(resolution))
	if v, ok := streamingResolutionMax[resolution]; ok {
		return v
	}
	return 0
}

func scaleFilter(vf *streamVideoFile, maxTranscodeSize int) string {
	if vf == nil || maxTranscodeSize == 0 || vf.Width <= 0 || vf.Height <= 0 {
		return ""
	}
	videoSize := vf.Height
	if vf.Width < videoSize {
		videoSize = vf.Width
	}
	if maxTranscodeSize >= videoSize {
		return ""
	}

	scaleFactor := float64(maxTranscodeSize) / float64(videoSize)
	width := int(math.Round(float64(vf.Width) * scaleFactor))
	height := int(math.Round(float64(vf.Height) * scaleFactor))
	if width%2 != 0 {
		width--
	}
	if height%2 != 0 {
		height--
	}
	if width <= 0 || height <= 0 {
		return ""
	}
	return fmt.Sprintf("scale=%d:%d", width, height)
}
