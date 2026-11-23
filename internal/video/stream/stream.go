package stream

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"math"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"

	"javboss/internal/common/logging"
	"javboss/internal/models"
	"javboss/internal/util"
)

const (
	MimeHLS    = "application/vnd.apple.mpegurl"
	MimeMpegTS = "video/MP2T"

	segmentLength = 2

	maxSegmentWait      = 15 * time.Second
	monitorInterval     = 200 * time.Millisecond
	maxSegmentGap       = 5
	resumeSegmentBuffer = 5
	maxSegmentBuffer    = 15
	maxIdleTime         = 2 * time.Minute

	resolutionParamKey = "resolution"
	apiKeyParamKey     = "apikey"
	locationIDParamKey = "location_id"
)

var ErrInvalidSegment = errors.New("invalid segment")

type streamer struct {
	cacheDir string

	context    context.Context
	cancelFunc context.CancelFunc

	runningStreams map[string]*runningStream
	streamsMutex   sync.Mutex
	watcher        *fsnotify.Watcher
	wake           chan struct{}
	processes      sync.WaitGroup
}

type Options struct {
	StreamType *StreamType
	SourcePath string
	Duration   float64
	MediaInfo  *models.MediaInfo
	Resolution string
	Key        string
	Segment    string
}

type StreamType struct {
	Name          string
	SegmentType   *SegmentType
	ServeManifest func(w http.ResponseWriter, r *http.Request, sourcePath string, durationHint float64, resolution string, mediaInfo *models.MediaInfo)
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

// Each batch starts at a single input seek point. Only that encoder timeline
// may extend its cached prefix, even when another seek overlaps the same indices.
type segmentBatch struct {
	first, end int // end is exclusive
	dir        string
	complete   bool
}

type transcodeProcess struct {
	batch       *segmentBatch
	control     *processControl
	ctx         context.Context
	paused      bool
	cancel      context.CancelFunc
	stagingDir  string
	nextSegment int
}

type waitingSegment struct {
	batch     *segmentBatch
	ctx       context.Context
	idx       int
	path      string
	accessed  time.Time
	available chan error
}

type runningStream struct {
	dir              string
	streamType       *StreamType
	vf               *streamVideoFile
	maxTranscodeSize int
	outputDir        string

	batches         []*segmentBatch
	selected        *segmentBatch
	prefetch        bool
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
		Name:          "hls",
		SegmentType:   SegmentTypeTS,
		ServeManifest: serveHLSManifest,
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
				"-hls_flags", "split_by_time+temp_file",
				"-hls_segment_type", "mpegts",
				"-hls_playlist_type", "vod",
				"-hls_segment_filename", filepath.Join(outputDir, "%d.ts"),
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

func newStreamer(cacheDir string) *streamer {
	ctx, cancel := context.WithCancel(context.Background())
	return &streamer{
		cacheDir:       cacheDir,
		wake:           make(chan struct{}, 1),
		context:        ctx,
		cancelFunc:     cancel,
		runningStreams: make(map[string]*runningStream),
	}
}

func (sm *streamer) Start(parent context.Context) {
	// Filesystem events handle normal delivery. The ticker also covers filesystems
	// that cannot be watched or drop events.
	watcher, err := fsnotify.NewWatcher()
	if err == nil {
		sm.watcher = watcher
		go sm.watchSegments(watcher)
	}
	go func() {
		ticker := time.NewTicker(monitorInterval)
		defer ticker.Stop()
		for {
			select {
			case <-sm.wake:
				sm.monitorStreams()
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

// Drain watcher events independently of streamsMutex. On Windows, adding or
// removing a watch can wait for the event reader. One wake covers a burst of files.
func (sm *streamer) watchSegments(watcher *fsnotify.Watcher) {
	for {
		select {
		case event, ok := <-watcher.Events:
			if !ok {
				return
			}
			if !event.Has(fsnotify.Create) || filepath.Ext(event.Name) != ".ts" {
				continue
			}
		case _, ok := <-watcher.Errors:
			if !ok {
				return
			}
		}
		sm.wakeMonitor()
	}
}

func (sm *streamer) wakeMonitor() {
	select {
	case sm.wake <- struct{}{}:
	default:
	}
}

func (sm *streamer) Stop() {
	sm.streamsMutex.Lock()
	sm.cancelFunc()
	streams := sm.runningStreams
	sm.runningStreams = nil
	if sm.watcher != nil {
		sm.watcher.Close()
		sm.watcher = nil
	}
	for _, stream := range streams {
		stream.failWaiters(nil, context.Canceled)
	}
	sm.streamsMutex.Unlock()
	sm.processes.Wait()
	for _, stream := range streams {
		removeTranscodeFiles(stream)
	}
}

// ServeManifest builds an HLS playlist without starting a transcoding process.
func ServeManifest(w http.ResponseWriter, r *http.Request, sourcePath string, durationHint float64, resolution string, mediaInfo *models.MediaInfo) {
	StreamTypeHLS.ServeManifest(w, r, sourcePath, durationHint, resolution, mediaInfo)
}

func (sm *streamer) ServeSegment(w http.ResponseWriter, r *http.Request, options Options) {
	if options.MediaInfo != nil && options.MediaInfo.Complete() && options.MediaInfo.DurationSeconds > 0 {
		options.Duration = options.MediaInfo.DurationSeconds
	}
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
	path := filepath.Join(outputDir, name)

	sm.streamsMutex.Lock()

	if sm.context.Err() != nil {
		sm.streamsMutex.Unlock()
		http.Error(w, "streaming service stopped", http.StatusServiceUnavailable)
		return
	}
	stream := sm.runningStreams[dir]
	if stream == nil {
		stream = &runningStream{
			dir:              dir,
			streamType:       options.StreamType,
			maxTranscodeSize: maxTranscodeSize,
			outputDir:        outputDir,
			waitingSegments:  make([]*waitingSegment, 0, 10),
		}
		sm.runningStreams[dir] = stream
	}

	now := time.Now()
	stream.lastAccessed = now
	previous := stream.lastSegment
	stream.lastSegment = segment

	// A legacy flat cache can serve an initial request. Once a batch is
	// selected, even its pending segments must stay on that encoder timeline.
	cached := stream.selected == nil && segmentExists(path)
	if !cached {
		batch := stream.batchFor(segment, previous, name)
		stream.prefetch = stream.prefetch || stream.selected != batch
		stream.selected = batch
		path = filepath.Join(batch.dir, name)
		cached = segmentExists(path)
	}
	if cached {
		sm.wakeMonitor()
		sm.streamsMutex.Unlock()
	} else {
		if stream.vf == nil {
			stream.vf, err = loadStreamVideoFile(options.SourcePath, options.Duration, options.MediaInfo)
			if err != nil {
				sm.streamsMutex.Unlock()
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
		}
		waiting := &waitingSegment{batch: stream.selected, ctx: r.Context(), idx: segment, path: path, accessed: now, available: make(chan error, 1)}
		stream.waitingSegments = append(stream.waitingSegments, waiting)
		sm.updateStream(stream, now)
		sm.streamsMutex.Unlock()
		select {
		case <-r.Context().Done():
			sm.wakeMonitor()
			return
		case err := <-waiting.available:
			if err != nil {
				if errors.Is(err, context.Canceled) {
					http.Error(w, "streaming service stopped", http.StatusServiceUnavailable)
				} else {
					http.Error(w, err.Error(), http.StatusInternalServerError)
				}
				return
			}
		}
	}
	w.Header().Set("Content-Type", options.StreamType.SegmentType.MimeType)
	http.ServeFile(w, r, path)
}

func (s *runningStream) batchFor(segment, previous int, name string) *segmentBatch {
	if b := s.selected; b != nil && segment >= b.first {
		// Continue across an overlap with a retired batch on our own timeline.
		if segment < b.end && segmentExists(filepath.Join(b.dir, name)) {
			return b
		}
		// Restarting a batch replays from first, even if its cached prefix is long.
		next := b.first
		active := s.tp != nil && s.tp.batch == b && s.tp.ctx.Err() == nil
		if active {
			next = s.tp.nextSegment
		}
		if !b.complete && segment >= next && segment-next <= maxSegmentGap && (active || segment == previous+1) {
			return b
		}
	}
	for _, b := range s.batches {
		if segment == b.first {
			return b
		}
		if segment > b.first && segment < b.end {
			// A retired, partial batch can only be extended by replaying its prefix
			// from the original seek point. Bound that work for distant seeks.
			if (b.complete || segment-b.first <= maxSegmentGap) && segmentExists(filepath.Join(b.dir, name)) {
				return b
			}
		}
	}
	b := &segmentBatch{first: segment, end: segment, dir: filepath.Join(s.outputDir, fmt.Sprintf("from-%d", segment))}
	s.batches = append(s.batches, b)
	return b
}

// The caller holds streamsMutex. Each process writes to its own staging directory
// so overlapping seeks cannot overwrite an already published segment.
func (sm *streamer) startTranscode(stream *runningStream, batch *segmentBatch) error {
	segment := batch.first
	ffmpegPath, err := util.ResolveFFmpegPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(batch.dir, 0755); err != nil {
		return err
	}
	staging, err := os.MkdirTemp(stream.outputDir, ".transcode-")
	if err != nil {
		return err
	}
	if sm.watcher != nil {
		// A failed watch is covered by the periodic scan.
		_ = sm.watcher.Add(staging)
	}
	ctx, cancel := context.WithCancel(sm.context)
	args := stream.makeStreamArgs(segment, staging)
	cmd := util.BackgroundCommandContext(ctx, ffmpegPath, args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Start(); err != nil {
		cancel()
		if sm.watcher != nil {
			_ = sm.watcher.Remove(staging)
		}
		os.RemoveAll(staging)
		return fmt.Errorf("start ffmpeg: %w", err)
	}
	control, err := openProcessControl(cmd.Process)
	if err != nil {
		cancel()
		_ = cmd.Wait()
		if sm.watcher != nil {
			_ = sm.watcher.Remove(staging)
		}
		os.RemoveAll(staging)
		return fmt.Errorf("open ffmpeg process: %w", err)
	}
	tp := &transcodeProcess{batch: batch, control: control, ctx: ctx, cancel: cancel, stagingDir: staging, nextSegment: segment}
	stream.tp = tp
	sm.processes.Add(1)
	go func() {
		defer sm.processes.Done()
		err := cmd.Wait()
		sm.streamsMutex.Lock()
		defer sm.streamsMutex.Unlock()
		defer cancel()
		defer control.close()
		defer os.RemoveAll(staging)
		if sm.watcher != nil {
			_ = sm.watcher.Remove(staging)
		}
		if sm.runningStreams[stream.dir] != stream {
			return
		}
		stream.publishSegments()
		// Deliver completed segments even if a later write failed.
		stream.collectWaiters(time.Now())
		if ctx.Err() == nil {
			if err != nil {
				err = fmt.Errorf("ffmpeg transcode: %w: %s", err, strings.TrimSpace(stderr.String()))
				logging.Error("%v", err)
			} else {
				batch.complete = true
				err = errors.New("ffmpeg reached end of input before the requested segment")
			}
			stream.failWaiters(batch, err)
		}
		stream.tp = nil
		sm.updateStream(stream, time.Now())
	}()
	return nil
}

// publishSegments moves only completed HLS segments. FFmpeg's temp_file flag
// leaves an interrupted write under a .tmp name that is never served.
func (stream *runningStream) publishSegments() {
	tp := stream.tp
	if tp == nil {
		return
	}
	for {
		name := stream.streamType.SegmentType.MakeFilename(tp.nextSegment)
		staged := filepath.Join(tp.stagingDir, name)
		if !segmentExists(staged) {
			return
		}
		final := filepath.Join(tp.batch.dir, name)
		var err error
		if segmentExists(final) {
			err = os.Remove(staged)
		} else {
			err = os.Rename(staged, final)
		}
		if err != nil {
			logging.Error("publish HLS segment: %v", err)
			return
		}
		tp.nextSegment++
		tp.batch.end = max(tp.batch.end, tp.nextSegment)
	}
}

func (stream *runningStream) collectWaiters(now time.Time) {
	stream.waitingSegments = slices.DeleteFunc(stream.waitingSegments, func(w *waitingSegment) bool {
		switch {
		case w.ctx.Err() != nil:
		case segmentExists(w.path):
			w.available <- nil
		case now.Sub(w.accessed) >= maxSegmentWait:
			w.available <- fmt.Errorf("timed out waiting for segment %d", w.idx)
		default:
			return false
		}
		return true
	})
}

func (stream *runningStream) failWaiters(batch *segmentBatch, err error) {
	stream.waitingSegments = slices.DeleteFunc(stream.waitingSegments, func(w *waitingSegment) bool {
		if batch != nil && w.batch != batch {
			return false
		}
		w.available <- err
		return true
	})
}

func (tp *transcodeProcess) setPaused(pause bool) {
	if tp.paused == pause || tp.ctx.Err() != nil {
		return
	}
	if err := tp.control.setPaused(pause); err != nil {
		// A failed resume cannot make progress. Reap the child before retrying demand.
		logging.Error("change ffmpeg pause state: %v", err)
		tp.cancel()
		return
	}
	tp.paused = pause
}

func (sm *streamer) updateStream(stream *runningStream, now time.Time) {
	stream.publishSegments()
	stream.collectWaiters(now)
	tp := stream.tp
	if len(stream.waitingSegments) > 0 {
		// Satisfy the oldest still-live request first. Duplicate requests share the
		// process, and disconnected seeks are removed before selecting a target.
		batch := stream.waitingSegments[0].batch
		if tp == nil {
			stream.prefetch = false
			if err := sm.startTranscode(stream, batch); err != nil {
				stream.failWaiters(batch, err)
			}
		} else if tp.batch != batch || stream.waitingSegments[0].idx < tp.nextSegment {
			tp.cancel()
		} else {
			tp.setPaused(false)
		}
		return
	}
	idle := now.Sub(stream.lastAccessed) > maxIdleTime
	if !idle && stream.prefetch && stream.selected != nil {
		if tp != nil && tp.batch != stream.selected {
			tp.cancel()
			return
		}
		stream.prefetch = false
		if tp == nil && !stream.selected.complete && stream.vf != nil {
			if err := sm.startTranscode(stream, stream.selected); err != nil {
				logging.Error("prefetch HLS batch: %v", err)
			}
			return
		}
	}
	if tp != nil {
		ahead := tp.nextSegment - 1 - stream.lastSegment
		switch {
		case idle:
			tp.cancel()
		case ahead >= maxSegmentBuffer:
			tp.setPaused(true)
		case ahead <= resumeSegmentBuffer:
			tp.setPaused(false)
		}
	} else if idle {
		removeTranscodeFiles(stream)
		delete(sm.runningStreams, stream.dir)
	}
}

func (sm *streamer) monitorStreams() {
	sm.streamsMutex.Lock()
	defer sm.streamsMutex.Unlock()
	now := time.Now()
	for _, stream := range sm.runningStreams {
		sm.updateStream(stream, now)
	}
}

func removeTranscodeFiles(stream *runningStream) {
	if err := os.RemoveAll(stream.outputDir); err != nil {
		logging.Error("remove segment directory failed %s: %v", stream.outputDir, err)
	}
}

func serveHLSManifest(w http.ResponseWriter, r *http.Request, sourcePath string, durationHint float64, resolution string, mediaInfo *models.MediaInfo) {
	vf, err := loadStreamVideoFile(sourcePath, durationHint, mediaInfo)
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

func (s *runningStream) makeStreamArgs(segment int, outputDir string) []string {
	args := []string{"-hide_banner", "-loglevel", "error"}
	if segment > 0 {
		args = append(args, "-ss", strconv.Itoa(segment*segmentLength))
	}
	args = append(args, "-i", s.vf.Path)

	videoOnly := strings.TrimSpace(s.vf.AudioCodec) == ""
	videoFilter := scaleFilter(s.vf, s.maxTranscodeSize)
	args = append(args, s.streamType.Args(segment, videoFilter, videoOnly, outputDir)...)
	return args
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

func loadStreamVideoFile(sourcePath string, durationHint float64, meta *models.MediaInfo) (*streamVideoFile, error) {
	if meta == nil || !meta.Complete() {
		var err error
		meta, err = util.ProbeVideo(sourcePath)
		if err != nil {
			return nil, fmt.Errorf("probe video file: %w", err)
		}
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
