<h1 align="center">Pornboss</h1>

<p align="center">An all-in-one solution for local adult video collections: automatically scan folders, generate cover screenshots, detect JAV titles and fetch metadata, provide powerful search for videos and JAV titles, and play videos quickly through the bundled mpv player.</p>

<p align="center">
  <a href="https://github.com/Solr159/pornboss/releases"><img alt="Release" src="https://img.shields.io/github/v/release/Solr159/pornboss?display_name=tag"></a>
  <a href="https://github.com/Solr159/pornboss/stargazers"><img alt="Stars" src="https://img.shields.io/github/stars/Solr159/pornboss?style=social"></a>
  <a href="https://github.com/Solr159/pornboss/releases"><img alt="Platform" src="https://img.shields.io/badge/Platform-Windows%20%7C%20Linux%20%7C%20macOS-1E88E5"></a>
  <a href="https://go.dev/"><img alt="Go" src="https://img.shields.io/badge/Go-1.25%2B-00ADD8?logo=go&logoColor=white"></a>
</p>

<p align="center">
  <a href="./README.md">中文</a> | <a href="./README.en.md">English</a>
</p>

## What Is Pornboss?

Pornboss is a local web app that provides full-featured, automated management for local adult video collections, with especially strong support for managing and searching Japanese AV.

The browsing experience is somewhat similar to JavDb, JavBus, and JavLibrary, but Pornboss optimizes and extends that style for a more direct, powerful, and user-friendly local media library experience.

## Core Philosophy

- **Ready to use, with no external dependencies**: Pornboss includes all runtime dependencies it needs. Add your local folders, wait a short moment, and start using it right away.
<br>

- **Fully automated managed folder service**: Pornboss provides a managed service for local media folders. Data fetching, indexing, and updates after folder changes are all handled automatically by Pornboss.
<br>

- **Non-intrusive design**: Pornboss respects your media folders. It only reads folder contents and never modifies them, so you do not need to worry about generated junk such as `.nfo` files or cover images appearing inside your own folders. This also lets Pornboss work alongside other video managers without interfering with them.
<br>

- **Data that stays safe**: All Pornboss data is stored in the project's `data/` directory. As long as you keep `data/` safe, your library data remains available when upgrading or moving to another computer.

## ✨ Features

### 1. 🔎 Powerful JAV Metadata Collection And Search

Pornboss extracts JAV codes from filenames, including common patterns such as `IPX-633`, `SSIS-001`, and `ipx633_ch`, then places recognized videos into the JAV library.

- Integrates multiple data sources internally, including javbus, avmoo, theporndb, and javdatabase, and fetches each type of information from the most suitable source.
- Automatically fetches title, release date, cover art, actresses, tags, and other basic metadata.
- Automatically fetches and completes actress profiles, including height, Chinese and English names, measurements, birthday, and more.
- Automatically fetches and completes JAV maker and series information.
- Supports Chinese and English JAV metadata fetching, freely switchable.
- Provides powerful sorting for JAV titles and actresses, including release date, duration, play count, height, age, measurements, and more.
- Provides powerful search and filtering, with editable complex queries covering keywords, actresses, tags, makers, series, and more for paged browsing.
- Provides powerful random browsing, including global random mode and random results under the current filters, with the ability to exit random mode at any time.

### 2. 📁 Smart Folder Management And Portable Data

After you add local media folders, Pornboss continuously syncs their contents in the background. Folder changes are detected and refreshed promptly, so newly added, removed, or moved files are reflected in the media library. Indexed videos can be browsed immediately while scanning and metadata completion continue in the background.

- Supports multiple media folders, including local disks, NAS mounts, and removable drives.
- Automatically generates video cover screenshots, stores video fingerprints, and tries to associate videos with JAV codes based on filenames.
- Lets you freely choose enabled folders, with disabled folder content hidden automatically.
- Keeps historical index data when a folder is temporarily unavailable, so removable-drive libraries reappear after the drive is connected again.
- Binds tags, JAV associations, and metadata to video fingerprints, so common move and rename workflows do not require retagging.
- Stores the database, covers, thumbnails, and runtime data under `data/`; copy this directory to upgrade or migrate.

### 3. ⏯️ Built-In mpv Playback

Pornboss integrates [mpv](https://github.com/mpv-player/mpv) playback, so clicking a video can launch a lightweight, high-performance local player that handles large files, high bitrates, and many common video formats.

- Plays the original local file through mpv, avoiding browser playback format limitations.
- Supports playback options such as default volume, window size, and always-on-top behavior.
- Supports custom hotkeys for actions such as seeking and volume adjustment.
- Bundles the [ModernZ](https://github.com/Samillion/ModernZ) OSC script, so mpv playback uses a more modern on-screen player UI by default.
- Supports taking screenshots at any moment during mpv playback, with screenshot files stored under `data/`.
- In both the video library and JAV library, the screenshot panel previews all mpv screenshots in timestamp order.
- The screenshot panel supports enlarged previews, deleting screenshots, and resuming playback directly from a screenshot timestamp.
- Lets you choose the default player in global settings, supports playback through mpv or the system player, and can reveal the file in its containing folder.

### 4. 🧭 Simple, Practical UI

The frontend is designed around finding the right video quickly. Common operations are centered on filtering, sorting, tagging, and random discovery instead of dense configuration screens.

- Supports a general video library, a JAV title library, and actress-centric browsing.
- Supports search, tag filters, multi-select batch tagging, and bulk tag replacement.
- Supports random browsing so older forgotten videos can surface again.
- Supports sorting by recently added, filename, duration, release date, play count, and more.

## Quick Start

### 1. Download

Go to the [Releases](https://github.com/Solr159/pornboss/releases) page, download the package for your system, and extract it:

- `windows-x86_64`
- `linux-x86_64`
- `macos-x86_64` for Intel-based macOS
- `macos-arm64` for Apple Silicon macOS

### 2. Start The App

- Windows: double-click `pornboss.exe`. If SmartScreen blocks it on first launch, click "More info" and continue.
- macOS: double-click `pornboss.command`. If macOS cannot verify the file, open `System Settings` -> `Privacy & Security`, scroll to the bottom, and click `Open Anyway`.

- Linux: open a terminal and run `pornboss`.

After launch, Pornboss will try to open your browser automatically. If it does not, open the local address shown in the terminal manually. Keep the terminal window open while Pornboss is running.

The release package includes a `config.toml` file in its root directory. By default `port = 0`, so Pornboss uses a random startup port. To use a fixed port, set it like this:

```text
port = 17654
```

### 3. Add Your Local Folders

Open `Global Settings` -> `Directory Management`, then add the local folders that store your videos. Scanning runs in the background, and indexed videos are available immediately without waiting for the full scan to finish. JAV mode updates may lag slightly behind video mode, so wait 1-5 minutes when importing new JAV titles.

If needed, open `Global Settings` -> `JAV Metadata`, switch the metadata language to `English`, and save.

### 4. Start Using It

- Choose the folders you want to browse from the top folder dropdown or `Directory Management`.
- Manage general adult videos, short clips, and compilations in `Video` mode.
- Browse JAV titles by title, actress, maker, series, and tag in `JAV` mode.
- Add custom tags such as `favorite`, `subtitled`, `uncensored`, or `must-watch`.
- Use search, filters, sorting, and random browsing to find content quickly.

## Screenshots

<p align="center">
  <img src="screenshot/en/image1_1.png" style="width: 100%; height: auto;">
</p>

<p align="center">
  <img src="screenshot/en/image1_2.png" style="width: 100%; height: auto;">
</p>

<p align="center">
  <img src="screenshot/en/image1_3.png" style="width: 100%; height: auto;">
</p>

<p align="center">
  <img src="screenshot/en/image1_4.png" style="width: 100%; height: auto;">
</p>

<p align="center">
  <img src="screenshot/en/image1_5.png" style="width: 100%; height: auto;">
</p>

<p align="center">
  <img src="screenshot/en/image1_6.png" style="width: 100%; height: auto;">
</p>

<p align="center">
  <img src="screenshot/en/image1_7.png" style="width: 100%; height: auto;">
</p>

<p align="center">
  <img src="screenshot/en/image1_8.png" style="width: 100%; height: auto;">
</p>

<p align="center">
  <img src="screenshot/en/image1_9.png" style="width: 100%; height: auto;">
</p>

<p align="center">
  <img src="screenshot/en/image1_10.png" style="width: 100%; height: auto;">
</p>

<p align="center">
  <img src="screenshot/en/image1_11.png" style="width: 100%; height: auto;">
</p>

<p align="center">
  <img src="screenshot/en/image1_12.png" style="width: 100%; height: auto;">
</p>

## How To Upgrade Versions

After downloading and extracting a new version, copy the old version's `data/` directory into the new version directory. Keep the old version and a data backup until you confirm the new version runs correctly.

## Notes

- Pornboss is a local media library manager, not an online streaming site.
- Initial JAV metadata and cover fetching depend on external website availability. If access is restricted in your region, prepare a working network/proxy environment yourself.
- When importing a large library for the first time, scanning, cover downloads, metadata completion, and thumbnail generation can take some time.

## Q&A

- Q: Why is Pornboss a local web app instead of a desktop app?
- A: This is not a technical limitation. It is mainly a user experience choice. For example, browsers have several unique advantages:
  1. If you want to view JAV titles from actress A and actress B while searching for videos containing keyword C, you can simply open multiple browser tabs.
  2. If you want to open a new page without losing the current page, use Ctrl + click or right-click and choose to open it in a new tab.
  3. If you click the wrong thing, the browser back button takes you back immediately.
  4. If you see a JAV title or actress name and want to search for it, select the text, right-click, and search it with Google.

<br>

- Q: Do I need to keep external network access available while using Pornboss?
- A: No. Pornboss reads all existing information from the `data/` directory, and anything you have already seen remains available offline. Without external network access, Pornboss cannot continue fetching or updating JAV information, but already indexed information is not affected.

<br>

- Q: After adding a folder, how do I know when scanning is finished? Do I need to wait?
- A: No. Pornboss scans and completes metadata in the background, so you can start using it right after adding a folder. You can also close the app at any time; scanning will continue the next time it starts.

<br>

- Q: After adding a folder, why do my JAV videos appear in regular video mode?
- A: This is expected. JAV metadata fetching has some delay compared with video scanning, so JAV videos may first appear as regular videos. If external network access is working, wait a moment and they will disappear from regular video mode and appear in JAV mode.

<br>

- Q: How do I add newly downloaded videos or remove videos I no longer want?
- A: Move videos into or out of a managed folder. Pornboss syncs folder state, so additions, moves, and removals are reflected in the library.

<br>

- Q: My video folder is on a removable drive. Will data be lost if I start Pornboss without the drive connected?
- A: No. When a folder is unavailable, Pornboss keeps the indexed data. The library will reappear after the drive is connected again.

<br>

- Q: One removable drive is running out of space. What should I do if I need to move the folder to a new drive?
- A: Move the folder directly, then update its path in `Directory Management`. You do not need to worry about data loss; Pornboss will handle it.

<br>

- Q: How do I migrate to another computer?
- A: For the same operating system, copy the entire Pornboss directory to the new computer and run it directly. For cross-platform migration, download the matching Pornboss package on the new computer, then copy the old computer's `data/` directory into the new Pornboss directory. If your video folder paths also changed, update them manually in `Directory Management`.

## Developer Notes

### Development Dependencies

- Go `1.25.1` or later
- Node.js and npm

### Tech Stack

- Backend: Go + Gin + GORM + SQLite
- Frontend: React + Vite + Tailwind + Zustand
- Media probing: `ffprobe`
- Thumbnail screenshot generation: `ffmpeg` on macOS, `mpv` on other platforms
- Playback and manual screenshots: `mpv`

### Common Commands

Download dependencies (`ffprobe` + `mpv`, plus `ffmpeg` on macOS):

```bash
./scripts/cli.sh download linux-x86_64
```

Install frontend dependencies:

```bash
cd web
npm install
```

Start the backend:

```bash
./scripts/cli.sh dev backend
```

Start the frontend:

```bash
./scripts/cli.sh dev frontend
```

Frontend checks:

```bash
cd web
npm run lint
npm run build
```

Build a release:

```bash
scripts/cli.sh release linux-x86_64 v0.1.0
```

### Project Structure

```text
cmd/server             Go server entrypoint
cmd/javprovider        JAV metadata provider debug entrypoint
internal/common        Shared global state and configuration
internal/db            GORM queries and SQLite storage
internal/jav           JAV metadata and actress profile fetching
internal/manager       Cover download and screenshot jobs
internal/models        Data model definitions
internal/mpv           mpv playback, hotkey, and manual screenshot configuration
internal/server        HTTP API and static asset routing
internal/service       Folder scanning, JAV detection, metadata completion
internal/util          File, system, proxy, and video probing utilities
web/                   React + Tailwind frontend
scripts/cli            Development, dependency download, and release helper CLI
data/                  Runtime database, covers, thumbnails, and cache
```
