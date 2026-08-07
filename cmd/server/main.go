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
	"strings"
	"sync"
	"syscall"
	"time"

	"javboss/internal/cache"
	clientpkg "javboss/internal/client"
	"javboss/internal/common"
	"javboss/internal/common/logging"
	"javboss/internal/db"
	"javboss/internal/jav"
	"javboss/internal/models"
	"javboss/internal/runtimeconfig"
	"javboss/internal/server"
	"javboss/internal/service"
	"javboss/internal/util"

	"javboss/internal/manager"

	"github.com/gin-gonic/gin"
	"github.com/pelletier/go-toml/v2"
	"golang.org/x/term"
	"gopkg.in/natefinch/lumberjack.v2"
)

var buildMode = "development"

const (
	defaultDevelopmentPort = 17654
	defaultReleasePort     = 8655
	defaultStaticDir       = "web/dist"
)

func main() {
	serverURLFlag := flag.String("server-url", "", "Remote JavBoss Server URL (enables Client mode)")
	serverPortFlag := flag.Int("port", 0, "Listening port (overrides config.toml)")
	flag.Parse()

	_ = os.Setenv("JAVBOSS_BUILD_MODE", buildMode)

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
	logging.SetLogger(logger)
	logging.SetColorEnabled(false)

	bootstrapCfg, err := clientpkg.LoadBootstrapConfig(baseDir)
	if err != nil {
		logger.Fatalf("load bootstrap config: %v", err)
	}
	portOverride, err := normalizePortOverride(*serverPortFlag)
	if err != nil {
		logger.Fatalf("resolve listening port: %v", err)
	}
	serverURL := resolveClientServerURL(*serverURLFlag, bootstrapCfg.ServerURL)
	if shouldRunClientMode(serverURL) {
		ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
		defer stop()
		clientPort := configuredPortWithOverride(bootstrapCfg.Port, portOverride)
		if err := runClientMode(ctx, stop, baseDir, serverURL, clientPort, logger); err != nil {
			logger.Fatalf("run client mode: %v", err)
		}
		return
	}

	cfg, err := common.LoadWithBaseDir(baseDir)
	if err != nil {
		logger.Fatalf("load config: %v", err)
	}

	if buildMode == "release" {
		dataDir := filepath.Dir(cfg.DatabasePath)
		lockPath := filepath.Join(dataDir, "javboss.lock")
		lock, ok := acquireSingleInstanceLock(lockPath, logger)
		if !ok {
			return
		}
		defer releaseFileLock(lock, lockPath, false, logger)

		// If a pre-rename lock file is still present, honor it during migration and
		// clean it up on normal exit. Do not recreate it once it has been removed.
		legacyLockPath := filepath.Join(dataDir, "pornboss.lock")
		legacyLock, ok := acquireExistingSingleInstanceLock(legacyLockPath, logger)
		if !ok {
			return
		}
		if legacyLock != nil {
			defer releaseFileLock(legacyLock, legacyLockPath, true, logger)
		}
	}

	if err := common.MigrateLegacyDatabase(cfg); err != nil {
		logger.Fatalf("migrate legacy database: %v", err)
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
	passwordResetPath := filepath.Join(filepath.Dir(cfg.DatabasePath), server.PasswordResetFilename)
	passwordResetApplied, err := server.ApplyPasswordResetFile(ctx, passwordResetPath)
	if err != nil {
		logger.Fatalf("apply password reset file: %v", err)
	}
	if passwordResetApplied {
		logger.Printf("password reset applied; all existing sessions were revoked")
	}
	runtimeCfg := applyRuntimeConfig(ctx)
	allowLANAccess := configBool(runtimeCfg["allow_lan_access"]) && !runtimeconfig.ContainerMode()

	var activeDirs []models.Directory
	if dirs, err := db.ListDirectories(ctx); err == nil {
		activeDirs = dirs
		logger.Printf("directories configured: %d (启动时不自动扫描)", len(activeDirs))
	} else {
		logger.Printf("list directories on startup failed: %v", err)
	}

	dataDir := filepath.Dir(cfg.DatabasePath)
	screenshotManager := manager.NewScreenshotManager(dataDir, db.GetVideo)
	streamManager := manager.NewStreamManager(filepath.Join(dataDir, "cache", "streams"))
	ffmpegToolManager := manager.NewFFmpegToolManager(ctx, baseDir)
	coverManager := manager.NewCoverManager(cfg.JavCoverDir, []jav.Provider{
		jav.ProviderJavDatabase,
		jav.ProviderJavBus,
		jav.ProviderThePornDB,
		jav.ProviderAvsox,
	})

	common.AppConfig = cfg
	common.ScreenshotManager = screenshotManager
	common.CoverManager = coverManager
	common.StreamManager = streamManager
	common.FFmpegToolManager = ffmpegToolManager

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
	streamManager.Start(ctx)
	go func() {
		timer := time.NewTimer(5 * time.Second)
		defer timer.Stop()

		select {
		case <-ctx.Done():
			return
		case <-timer.C:
			service.StartAutomaticDirectoryScanScheduler(ctx, 30*time.Second)
			service.StartJavMetadataScanner(ctx, time.Minute)
			service.StartSlowJavMetadataScanner(ctx, time.Minute)
			service.StartIdolProfileScanner(ctx, time.Minute)
		}
	}()

	authService, err := server.NewAuthServiceForInstance(ctx, baseDir)
	if err != nil {
		logger.Fatalf("initialize authentication: %v", err)
	}

	router := server.NewRouter(resolveStaticDir(defaultStaticDir), authService)
	serverPort := defaultDevelopmentPort
	if portOverride > 0 {
		serverPort = portOverride
	}
	listenAddr := configuredListenAddr(
		serverPort,
		allowLANAccess,
		runtimeconfig.ContainerMode(),
	)

	srv := &http.Server{
		Addr:         listenAddr,
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

	if buildMode == "release" {
		listenAddr, err := releaseListenAddr(baseDir, allowLANAccess, portOverride)
		if err != nil {
			logger.Fatalf("resolve release listen address: %v", err)
		}
		listener, err := net.Listen("tcp", listenAddr)
		if err != nil {
			logger.Fatalf("listen on %s: %v", listenAddr, err)
		}
		actualPort := listener.Addr().(*net.TCPAddr).Port
		displayURL := fmt.Sprintf("http://localhost:%d", actualPort)
		openURL := displayURL
		printReleaseStartupHint(displayURL)
		if err := util.OpenFile(openURL); err != nil {
			logger.Printf("open browser failed: %v", err)
		}
		startReleaseKeyboardControls(ctx, stop, openURL, logger)
		logger.Printf("server listening on %s", listener.Addr().String())
		if err := srv.Serve(listener); err != nil && err != http.ErrServerClosed {
			logger.Fatalf("server error: %v", err)
		}
		return
	}

	logger.Printf("server listening on %s", listenAddr)
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		logger.Fatalf("server error: %v", err)
	}
}

func shouldRunClientMode(serverURL string) bool {
	return strings.TrimSpace(serverURL) != ""
}

func resolveClientServerURL(flagValue, configuredValue string) string {
	if value := strings.TrimSpace(flagValue); value != "" {
		return value
	}
	return strings.TrimSpace(configuredValue)
}

func runClientMode(ctx context.Context, stop context.CancelFunc, baseDir, serverURL string, configuredPort int, logger *log.Logger) error {
	port := configuredPort
	if port == 0 {
		if buildMode == "release" {
			port = defaultReleasePort
		} else {
			port = defaultDevelopmentPort
		}
	}
	if port < 1 || port > 65535 {
		return fmt.Errorf("invalid client port %d", port)
	}
	listenAddr := net.JoinHostPort("127.0.0.1", strconv.Itoa(port))
	localURL := fmt.Sprintf("http://127.0.0.1:%d", port)
	normalizedServerURL, err := clientpkg.NormalizeServerURL(serverURL)
	if err != nil {
		return fmt.Errorf("invalid configured remote server: %w", err)
	}
	serverURL = normalizedServerURL
	clientHandler, err := clientpkg.New(clientpkg.Options{
		BaseDir:      baseDir,
		LocalBaseURL: localURL,
		RemoteURL:    serverURL,
	})
	if err != nil {
		return err
	}
	defer clientHandler.Close()

	srv := &http.Server{
		Addr:              listenAddr,
		Handler:           clientHandler,
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       120 * time.Second,
	}
	listener, err := net.Listen("tcp", listenAddr)
	if err != nil {
		return fmt.Errorf("listen on %s: %w", listenAddr, err)
	}
	defer listener.Close()
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := srv.Shutdown(shutdownCtx); err != nil {
			logger.Printf("client server shutdown error: %v", err)
		}
	}()

	displayURL := fmt.Sprintf("http://localhost:%d", port)
	if buildMode == "release" {
		printReleaseClientStartupHint(displayURL, serverURL)
		if err := util.OpenFile(displayURL); err != nil {
			logger.Printf("open browser failed: %v", err)
		}
		startReleaseKeyboardControls(ctx, stop, displayURL, logger)
	}
	logger.Printf("client mode listening on %s, remote server %s", listenAddr, strings.TrimSpace(serverURL))
	if err := srv.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("client server: %w", err)
	}
	return nil
}

func applyRuntimeConfig(ctx context.Context) map[string]string {
	cfg, err := db.ListConfig(ctx)
	if err != nil {
		logging.Error("load runtime config failed: %v", err)
		return nil
	}
	util.SetProxyFromStrings(cfg["proxy_host"], cfg["proxy_port"])
	return cfg
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
		Filename:   filepath.Join(logsDir, "javboss.log"),
		MaxSize:    20, // megabytes
		MaxBackups: 7,
		MaxAge:     14, // days
		Compress:   true,
		LocalTime:  true,
	}

	logger := log.New(rotator, "", log.LstdFlags|log.Lmicroseconds)
	return logger, func() { _ = rotator.Close() }, nil
}

func configuredListenAddr(port int, allowLANAccess bool, containerMode bool) string {
	host := "127.0.0.1"
	if allowLANAccess || containerMode {
		host = "0.0.0.0"
	}
	return net.JoinHostPort(host, strconv.Itoa(port))
}

func normalizePortOverride(value int) (int, error) {
	if value == 0 {
		return 0, nil
	}
	if value < 1 || value > 65535 {
		return 0, fmt.Errorf("invalid port %d", value)
	}
	return value, nil
}

func configuredPortWithOverride(configured, override int) int {
	if override > 0 {
		return override
	}
	return configured
}

func releaseListenAddr(baseDir string, allowLANAccess bool, portOverride int) (string, error) {
	if portOverride > 0 {
		return configuredListenAddr(portOverride, allowLANAccess, false), nil
	}
	port, configured, err := releaseConfigPort(baseDir)
	if err != nil {
		return "", err
	}
	if configured {
		return configuredListenAddr(port, allowLANAccess, false), nil
	}

	return configuredListenAddr(defaultReleasePort, allowLANAccess, false), nil
}

func configBool(raw string) bool {
	value, err := strconv.ParseBool(raw)
	return err == nil && value
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
		fmt.Printf("JavBoss 启动成功，浏览器访问地址：%s\n", url)
		fmt.Println("按 1 打开新页面，按 2 或者关闭此窗口退出应用。")
		return
	}
	fmt.Printf("JavBoss started successfully. Browser URL: %s\n", url)
	fmt.Println("Press 1 to open a new page. Press 2 or close this window to exit the app.")
}

func printReleaseClientStartupHint(localURL, serverURL string) {
	fmt.Print(releaseClientStartupHint(localURL, serverURL, util.SystemPrefersChinese()))
}

func releaseClientStartupHint(localURL, serverURL string, chinese bool) string {
	remote := strings.TrimSpace(serverURL)
	if chinese {
		return fmt.Sprintf(
			"JavBoss 已通过 Client 模式启动，访问地址：%s\n远程 Server 地址：%s\n按 1 打开新页面，按 2 或者关闭此窗口退出应用。\n",
			localURL,
			remote,
		)
	}
	return fmt.Sprintf(
		"JavBoss started in Client mode. URL: %s\nRemote Server: %s\nPress 1 to open a new page. Press 2 or close this window to exit the app.\n",
		localURL,
		remote,
	)
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

func acquireSingleInstanceLock(path string, logger *log.Logger) (*util.FileLock, bool) {
	lock, err := util.AcquireFileLock(path)
	if err != nil {
		if errors.Is(err, util.ErrLockHeld) {
			fmt.Println("JavBoss 已在运行，无法重复启动。")
			waitForUserExit()
			return nil, false
		}
		logger.Fatalf("acquire lock %s: %v", path, err)
	}
	return lock, true
}

func acquireExistingSingleInstanceLock(path string, logger *log.Logger) (*util.FileLock, bool) {
	lock, err := util.AcquireExistingFileLock(path)
	if err != nil {
		if errors.Is(err, util.ErrLockMissing) {
			return nil, true
		}
		if errors.Is(err, util.ErrLockHeld) {
			fmt.Println("PornBoss 已在运行，无法重复启动。")
			waitForUserExit()
			return nil, false
		}
		logger.Fatalf("acquire lock %s: %v", path, err)
	}
	return lock, true
}

func releaseFileLock(lock *util.FileLock, path string, removeOnRelease bool, logger *log.Logger) {
	if err := lock.Release(); err != nil {
		logger.Printf("release lock failed: %v", err)
	}
	if removeOnRelease {
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			logger.Printf("remove legacy lock failed: %v", err)
		}
	}
}
