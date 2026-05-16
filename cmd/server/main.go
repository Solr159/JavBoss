package main

import (
	"bufio"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"sync"
	"syscall"
	"time"

	"pornboss/internal/cache"
	"pornboss/internal/common"
	"pornboss/internal/common/logging"
	"pornboss/internal/db"
	"pornboss/internal/jav"
	"pornboss/internal/models"
	"pornboss/internal/server"
	"pornboss/internal/service"
	"pornboss/internal/util"

	"pornboss/internal/manager"

	"github.com/gin-gonic/gin"
	"github.com/pelletier/go-toml/v2"
	"golang.org/x/term"
	"gopkg.in/natefinch/lumberjack.v2"
)

var buildMode = "development"

func main() {
	addr := flag.String("addr", ":17654", "HTTP address to listen on")
	staticDir := flag.String("static", "web/dist", "Path to built frontend assets")
	flag.Parse()

	_ = os.Setenv("PORNBOSS_BUILD_MODE", buildMode)

	if buildMode == "release" && os.Getenv("GIN_MODE") == "" {
		gin.SetMode(gin.ReleaseMode)
	}

	baseDir, err := resolveBaseDir()
	if err != nil {
		fallback := log.New(os.Stderr, "", log.LstdFlags|log.Lmicroseconds)
		fallback.Fatalf("resolve base dir: %v", err)
	}

	logger, closeLogs, err := buildLogger(baseDir)
	if err != nil {
		fallback := log.New(os.Stderr, "", log.LstdFlags|log.Lmicroseconds)
		fallback.Fatalf("init logger: %v", err)
	}
	defer closeLogs()

	cfg, err := common.LoadWithBaseDir(baseDir)
	if err != nil {
		logger.Fatalf("load config: %v", err)
	}

	logging.SetLogger(logger)
	logging.SetColorEnabled(false)

	if buildMode == "release" {
		lockPath := filepath.Join(filepath.Dir(cfg.DatabasePath), "pornboss.lock")
		lock, err := util.AcquireFileLock(lockPath)
		if err != nil {
			if errors.Is(err, util.ErrLockHeld) {
				fmt.Println("Pornboss 已在运行，无法重复启动。")
				waitForUserExit()
				return
			}
			logger.Fatalf("acquire lock: %v", err)
		}
		defer func() {
			if err := lock.Release(); err != nil {
				logger.Printf("release lock failed: %v", err)
			}
		}()
	}

	database, err := db.Open(cfg.DatabasePath)
	if err != nil {
		logger.Fatalf("open database: %v", err)
	}
	sqlDB, err := database.DB()
	if err != nil {
		logger.Fatalf("database handle: %v", err)
	}
	defer sqlDB.Close()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	common.DB = database
	applyRuntimeConfig(ctx)

	var activeDirs []models.Directory
	if dirs, err := db.ListDirectories(ctx); err == nil {
		activeDirs = dirs
		logger.Printf("directories configured: %d (启动时不自动扫描)", len(activeDirs))
	} else {
		logger.Printf("list directories on startup failed: %v", err)
	}

	dataDir := filepath.Dir(cfg.DatabasePath)
	screenshotManager := manager.NewScreenshotManager(dataDir, db.GetVideo)
	coverManager := manager.NewCoverManager(cfg.JavCoverDir, []jav.Provider{
		jav.ProviderJavDatabase,
		jav.ProviderJavBus,
		jav.ProviderThePornDB,
	})

	common.AppConfig = cfg
	common.ScreenshotManager = screenshotManager
	common.CoverManager = coverManager

	javCache, err := cache.OpenSQLiteKV(filepath.Join(dataDir, "cache", "jav_cache.db"))
	if err != nil {
		logger.Printf("open jav lookup cache failed, continue without cache: %v", err)
	} else {
		defer javCache.Close()
		jav.SetCache(javCache)
		javCache.StartCleaner(ctx, 24*time.Hour)
	}

	screenshotManager.Start(ctx)
	coverManager.Start(ctx)
	service.StartDirectoryScanner(ctx, time.Minute)
	service.StartJavScanner(ctx, time.Minute)
	service.StartJavMetadataScanner(ctx, time.Minute)
	service.StartIdolProfileScanner(ctx, time.Minute)

	router := server.NewRouter(resolveStaticDir(*staticDir))

	srv := &http.Server{
		Addr:         *addr,
		Handler:      router,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := srv.Shutdown(shutdownCtx); err != nil {
			logger.Printf("server shutdown error: %v", err)
		}
	}()

	if gin.Mode() == gin.ReleaseMode {
		listenAddr, err := releaseListenAddr(*addr, baseDir)
		if err != nil {
			logger.Fatalf("resolve release listen address: %v", err)
		}
		listener, err := net.Listen("tcp", listenAddr)
		if err != nil {
			logger.Fatalf("listen on %s: %v", listenAddr, err)
		}
		actualPort := listener.Addr().(*net.TCPAddr).Port
		url := fmt.Sprintf("http://localhost:%d", actualPort)
		printReleaseStartupHint(url)
		if err := util.OpenFile(url); err != nil {
			logger.Printf("open browser failed: %v", err)
		}
		startReleaseKeyboardControls(ctx, stop, url, logger)
		logger.Printf("server listening on %s", listener.Addr().String())
		if err := srv.Serve(listener); err != nil && err != http.ErrServerClosed {
			logger.Fatalf("server error: %v", err)
		}
		return
	}

	logger.Printf("server listening on %s", *addr)
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		logger.Fatalf("server error: %v", err)
	}
}

func applyRuntimeConfig(ctx context.Context) {
	cfg, err := db.ListConfig(ctx)
	if err != nil {
		logging.Error("load runtime config failed: %v", err)
		return
	}
	util.SetProxyFromStrings(cfg["proxy_host"], cfg["proxy_port"])
	jav.SetMetadataLanguage(cfg["jav_metadata_language"])
}

func buildLogger(baseDir string) (*log.Logger, func(), error) {
	if gin.Mode() != gin.ReleaseMode {
		logger := log.New(os.Stdout, "", log.LstdFlags|log.Lmicroseconds)
		return logger, func() {}, nil
	}

	logsDir := filepath.Join(baseDir, "logs")
	if err := os.MkdirAll(logsDir, 0o755); err != nil {
		return nil, nil, fmt.Errorf("create logs dir: %w", err)
	}

	rotator := &lumberjack.Logger{
		Filename:   filepath.Join(logsDir, "pornboss.log"),
		MaxSize:    20, // megabytes
		MaxBackups: 7,
		MaxAge:     14, // days
		Compress:   true,
		LocalTime:  true,
	}

	logger := log.New(rotator, "", log.LstdFlags|log.Lmicroseconds)
	return logger, func() { _ = rotator.Close() }, nil
}

func releaseListenAddr(addr string, baseDir string) (string, error) {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		host = ""
	}

	port, configured, err := releaseConfigPort(baseDir)
	if err != nil {
		return "", err
	}
	if configured {
		if host == "" {
			return net.JoinHostPort("", strconv.Itoa(port)), nil
		}
		return net.JoinHostPort(host, strconv.Itoa(port)), nil
	}

	if host == "" {
		return ":0", nil
	}
	return net.JoinHostPort(host, "0"), nil
}

func releaseConfigPort(baseDir string) (int, bool, error) {
	if baseDir == "" {
		return 0, false, nil
	}
	data, err := os.ReadFile(filepath.Join(baseDir, "config.toml"))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return 0, false, nil
		}
		return 0, false, fmt.Errorf("read config: %w", err)
	}

	var cfg struct {
		Port int `toml:"port"`
	}
	if err := toml.Unmarshal(data, &cfg); err != nil {
		return 0, false, fmt.Errorf("parse config TOML: %w", err)
	}
	if cfg.Port == 0 {
		return 0, false, nil
	}
	if cfg.Port < 1 || cfg.Port > 65535 {
		return 0, false, fmt.Errorf("invalid config port %d", cfg.Port)
	}
	return cfg.Port, true, nil
}

func printReleaseStartupHint(url string) {
	if util.SystemPrefersChinese() {
		fmt.Printf("Pornboss启动成功，浏览器访问地址：%s\n", url)
		fmt.Println("按 1 打开新页面，按 2 或者关闭此窗口退出应用。")
		return
	}
	fmt.Printf("Pornboss started successfully. Browser URL: %s\n", url)
	fmt.Println("Press 1 to open a new page. Press 2 or close this window to exit the app.")
}

func startReleaseKeyboardControls(ctx context.Context, cancel context.CancelFunc, url string, logger *log.Logger) {
	fd := int(os.Stdin.Fd())
	restoreTerminal := func() {}
	var restoreOnce sync.Once
	if term.IsTerminal(fd) {
		oldState, err := term.MakeRaw(fd)
		if err != nil {
			logger.Printf("enable release keyboard controls raw mode failed: %v", err)
		} else {
			restoreTerminal = func() {
				if err := term.Restore(fd, oldState); err != nil {
					logger.Printf("restore terminal failed: %v", err)
				}
			}
			go func() {
				<-ctx.Done()
				restoreOnce.Do(restoreTerminal)
			}()
		}
	}

	go func() {
		defer restoreOnce.Do(restoreTerminal)
		reader := bufio.NewReader(os.Stdin)
		for {
			b, err := reader.ReadByte()
			if err != nil {
				if ctx.Err() == nil && !errors.Is(err, io.EOF) {
					logger.Printf("release keyboard controls stopped: %v", err)
				}
				return
			}
			switch b {
			case '1':
				if err := util.OpenFile(url); err != nil {
					logger.Printf("open browser from keyboard control failed: %v", err)
				}
			case '2', 3:
				cancel()
				return
			case '\r', '\n':
			default:
			}
		}
	}()
}

func resolveBaseDir() (string, error) {
	if buildMode == "release" {
		if execPath, err := os.Executable(); err == nil {
			return filepath.Dir(execPath), nil
		}
	}
	if wd, err := os.Getwd(); err == nil {
		return wd, nil
	}
	if execPath, err := os.Executable(); err == nil {
		return filepath.Dir(execPath), nil
	}
	return "", fmt.Errorf("unable to resolve base directory")
}

func resolveStaticDir(staticDir string) string {
	if staticDir == "" {
		return ""
	}
	if fi, err := os.Stat(staticDir); err == nil && fi.IsDir() {
		return staticDir
	}
	if filepath.IsAbs(staticDir) {
		return staticDir
	}
	execPath, err := os.Executable()
	if err != nil {
		return staticDir
	}
	execDir := filepath.Dir(execPath)
	candidate := filepath.Join(execDir, staticDir)
	if fi, err := os.Stat(candidate); err == nil && fi.IsDir() {
		return candidate
	}
	return staticDir
}

func waitForUserExit() {
	fmt.Println("请手动关闭此窗口，或按回车键退出。")
	reader := bufio.NewReader(os.Stdin)
	if _, err := reader.ReadString('\n'); err != nil {
		select {}
	}
}
