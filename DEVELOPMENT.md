# JavBoss 开发者说明

## 开发环境依赖

- Go `1.25.1` 或更高版本
- Node.js 24 和 npm（与 PR 检查和 Docker 构建一致）

## 技术栈

- Backend: Go + Gin + GORM + SQLite
- Frontend: React + Vite + Tailwind + Zustand
- 媒体探测: `ffprobe`
- 缩略图截图生成: macOS 使用 `ffmpeg`，其他平台使用 `mpv`
- 播放与手动截图: `mpv`

## 常用命令

下载依赖（`ffprobe` + `mpv`，macOS 额外下载 `ffmpeg`）：

```bash
./scripts/cli.sh download linux-x86_64
```

安装前端依赖：

```bash
cd web
npm install
```

启动后端：

```bash
./scripts/cli.sh dev backend
```

按 Docker 运行时配置启动本地后端（用于调试容器模式行为）：

```bash
DOCKER_MODE=1 ./scripts/cli.sh dev backend
```

启动前端：

```bash
./scripts/cli.sh dev frontend
```

前端检查：

```bash
cd web
npm test
npm run lint
npm run format:check
npm run build
```

打包发布：

```bash
scripts/cli.sh release linux-x86_64 v0.1.0
```

## 通过 CLI 启停 Docker

需要 Node.js/npm、Docker 和 Docker Compose 插件。在仓库根目录执行：

```bash
# 构建镜像并后台启动容器
./scripts/cli.sh docker start

# 停止容器，保留容器和数据
./scripts/cli.sh docker stop
```

也可运行 `./scripts/cli.sh`，在第一层选择 `docker`，再选择 `start` 或 `stop`。

`start` 使用 `compose.local.yaml` 在 Docker 内构建前后端，默认镜像为 `javboss:local`，访问地址为 `http://localhost:5174`。数据保存在仓库根目录的 `docker-data/`，宿主机根目录只读挂载到 `/host`。重复启动会重新构建镜像并按需更新容器，继续使用该数据目录，不会自动迁移其他目录的数据。

更多参数可通过 `./scripts/cli.sh docker --help` 查看。

## 项目结构

```text
cmd/server             Go 服务入口
cmd/javprovider        JAV 元数据 provider 调试入口
internal/common        共享路径与运行环境配置、版本与日志
internal/db            GORM 模型查询、SQLite 存储与应用默认 Store
internal/jav/scrape    JAV 元数据与女优资料抓取、provider、查询缓存
internal/jav/scrape/cover JAV 封面下载与缓存
internal/jav/translation 标题翻译
internal/jav/enrichment 元数据后台补全
internal/download      下载任务调度与文件保存
internal/download/downloader 下载器客户端与协议适配
internal/download/clouddrive CloudDrive 客户端与协议定义
internal/video/screenshot 视频截图与缩略图队列
internal/video/stream  视频流媒体转码
internal/ffmpeg        FFmpeg 安装与更新
internal/models        数据模型定义
internal/mpv           mpv 播放、快捷键与手动截图配置
internal/api           HTTP API 与静态资源路由
internal/directory     目录扫描、文件整理与 JAV 关联
internal/util          文件、系统、代理、视频探测等工具
web/                   React + Tailwind 前端
scripts/cli            开发、依赖下载与发布辅助 CLI
data/                  运行期数据库、封面、缩略图与缓存
```

后端使用单一应用数据库：启动时通过 `db.SetDefault(db.NewStore(database))` 设置默认 Store，业务代码通过 `db.Default()` 访问，无需逐层传参。初始化必须早于 HTTP 服务和后台任务；数据库由入口负责关闭。替换默认 Store 的测试应串行运行，等待后台任务结束后恢复原值。

封面、截图、流媒体、下载任务和 FFmpeg 安装通过业务包函数调用，例如 `cover.Enqueue(...)`、`screenshot.GetThumbnail(...)`、`download.Wake()` 和 `ffmpeg.GetStatus()`，不需要构造或传递默认实例。入口在消费者启动前调用 `cover.Start(ctx, coverDir)`、`screenshot.Start(ctx, dataDir)`、`stream.Start(ctx, cacheDir)`、`download.Start(ctx)` 和 `ffmpeg.Init(ctx, baseDir)`；队列、锁、进程和下载进度由包内私有结构保存，后台任务随 context 取消退出。启动配置只设置一次，不支持运行时替换；空封面或截图目录会停用对应队列。`cover.DownloadFromURL`、`cover.FindPath`、`screenshot.CaptureFile` 和 `stream.ServeManifest` 是独立函数，无需启动后台队列。

`internal/jav` 只按抓取、翻译和后台补全划分为 `scrape`、`translation`、`enrichment` 三个子目录。抓取入口使用 `scrape.LookupJavByCode`、`scrape.ResolveJavByCodes` 等包函数；各站点实现、封面抓取、共享数据类型（`scrape/metadata`）和内部解析工具都归属于 `scrape`。`enrichment` 调用 `scrape` 获取补全数据；数据库仅引用 `scrape/metadata` 中的数据类型，无需依赖抓取客户端。

`internal/api` 的路由注册、HTTP 处理函数和辅助函数使用普通包函数，通过 `api.NewRouter(...)` 和 `api.ServeHTTP(...)` 启动，不再构造 `Server`。路径配置由 `common.SetConfig(cfg)` 初始化，通过 `common.CurrentConfig()` 读取副本；目录扫描状态由 `directory` 管理，播放会话和观看时长订阅由 `playback` 管理，翻译批处理和并发限制由 `jav/translation` 管理，资源采样由 `monitor` 管理。API 只保留鉴权和监听地址切换等 HTTP 职责。

入口在消费者启动前调用 `directory.Init(ctx, coverDir)` 和 `translation.Init(ctx)`，退出时先取消应用 context，再调用 `translation.WaitBatches()`，最后关闭数据库。业务包内部保留维护锁、队列和会话所需的状态；调用方无需构造或传递这些实例。应用采用单实例运行方式，测试替换共享配置或数据库时必须串行执行，并等待后台任务退出。
