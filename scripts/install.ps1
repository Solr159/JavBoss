$ErrorActionPreference = "Stop"
$ProgressPreference = "Continue"

$Version = if ($env:JAVBOSS_VERSION) { $env:JAVBOSS_VERSION } else { "latest" }
$Dir = $env:JAVBOSS_INSTALL_DIR
$Repo = if ($env:JAVBOSS_REPO) { $env:JAVBOSS_REPO } else { "Solr159/JavBoss" }
$NoStart = $env:JAVBOSS_NO_START -eq "1"

function Test-PrefersChinese {
  if ($env:JAVBOSS_LANG -like "zh*") {
    return $true
  }
  try {
    return [System.Globalization.CultureInfo]::CurrentUICulture.Name -like "zh*"
  } catch {
    return $false
  }
}

function Write-Log {
  param([string]$Message)
  Write-Host "[javboss] $Message"
}

function Fail {
  param([string]$Message)
  Write-Error "[javboss] ERROR: $Message"
  exit 1
}

function Format-Bytes {
  param([double]$Bytes)

  if ($Bytes -ge 1GB) {
    return "{0:N1} GB" -f ($Bytes / 1GB)
  }
  if ($Bytes -ge 1MB) {
    return "{0:N1} MB" -f ($Bytes / 1MB)
  }
  if ($Bytes -ge 1KB) {
    return "{0:N1} KB" -f ($Bytes / 1KB)
  }
  return "$Bytes B"
}

function Download-FileWithProgress {
  param([string]$Url, [string]$OutFile)

  Add-Type -AssemblyName System.Net.Http

  $client = [System.Net.Http.HttpClient]::new()
  $client.DefaultRequestHeaders.UserAgent.ParseAdd("JavBoss-Installer")
  $response = $null
  $inputStream = $null
  $outputStream = $null

  try {
    $response = $client.GetAsync($Url, [System.Net.Http.HttpCompletionOption]::ResponseHeadersRead).GetAwaiter().GetResult()
    $response.EnsureSuccessStatusCode() | Out-Null

    $total = $response.Content.Headers.ContentLength
    $inputStream = $response.Content.ReadAsStreamAsync().GetAwaiter().GetResult()
    $outputStream = [System.IO.File]::Open($OutFile, [System.IO.FileMode]::Create, [System.IO.FileAccess]::Write, [System.IO.FileShare]::None)
    $buffer = New-Object byte[] 4194304
    [long]$downloaded = 0

    while (($read = $inputStream.Read($buffer, 0, $buffer.Length)) -gt 0) {
      $outputStream.Write($buffer, 0, $read)
      $downloaded += $read

      if ($total) {
        $percent = [Math]::Min(100, [Math]::Floor(($downloaded * 100) / $total))
        Write-Progress -Activity "Downloading JavBoss" -Status "$(Format-Bytes $downloaded) / $(Format-Bytes $total)" -PercentComplete $percent
      } else {
        Write-Progress -Activity "Downloading JavBoss" -Status "$(Format-Bytes $downloaded)" -PercentComplete 0
      }
    }

    Write-Progress -Activity "Downloading JavBoss" -Completed
  } finally {
    if ($outputStream) {
      $outputStream.Dispose()
    }
    if ($inputStream) {
      $inputStream.Dispose()
    }
    if ($response) {
      $response.Dispose()
    }
    $client.Dispose()
  }
}

function Get-LatestTag {
  param([string]$Repository)
  $uri = "https://api.github.com/repos/$Repository/releases/latest"
  try {
    $release = Invoke-RestMethod -Uri $uri -Headers @{ "User-Agent" = "JavBoss-Installer" }
    if (-not $release.tag_name) {
      Fail "failed to read latest release tag from GitHub"
    }
    return [string]$release.tag_name
  } catch {
    Fail "failed to read latest release tag from GitHub: $($_.Exception.Message)"
  }
}

function Normalize-Tag {
  param([string]$InputVersion, [string]$Repository)
  if ([string]::IsNullOrWhiteSpace($InputVersion)) {
    $InputVersion = "latest"
  }
  if ($InputVersion -eq "latest") {
    return Get-LatestTag -Repository $Repository
  }
  if ($InputVersion -like "v*") {
    return $InputVersion
  }
  return "v$InputVersion"
}

function Get-PlatformLabel {
  $isWindowsValue = Get-Variable -Name IsWindows -ValueOnly -ErrorAction SilentlyContinue
  if (($PSVersionTable.PSEdition -eq "Core" -and $isWindowsValue -eq $false) -or ($env:OS -and $env:OS -ne "Windows_NT")) {
    Fail "scripts/install.ps1 is for Windows. Use scripts/install.sh on Linux or macOS"
  }

  if ([Environment]::Is64BitOperatingSystem) {
    return "windows-x86_64"
  }
  Fail "unsupported Windows architecture"
}

function Save-ExistingConfig {
  param([string]$InstallDir)
  $config = Join-Path $InstallDir "config.toml"
  if (-not (Test-Path -LiteralPath $config)) {
    return $null
  }
  $saved = Join-Path ([System.IO.Path]::GetTempPath()) ("javboss-config-" + [System.Guid]::NewGuid().ToString() + ".toml")
  Copy-Item -LiteralPath $config -Destination $saved -Force
  return $saved
}

function Restore-ExistingConfig {
  param([string]$SavedConfig, [string]$InstallDir)
  if (-not $SavedConfig) {
    return
  }
  Copy-Item -LiteralPath $SavedConfig -Destination (Join-Path $InstallDir "config.toml") -Force
  Remove-Item -LiteralPath $SavedConfig -Force -ErrorAction SilentlyContinue
}

function Get-VersionFilePath {
  param([string]$InstallDir)
  return Join-Path $InstallDir ".javboss-version"
}

function Get-InstalledVersion {
  param([string]$InstallDir)

  $exe = Join-Path $InstallDir "javboss.exe"
  $versionFile = Get-VersionFilePath -InstallDir $InstallDir
  if ((Test-Path -LiteralPath $exe) -and (Test-Path -LiteralPath $versionFile)) {
    $line = Get-Content -LiteralPath $versionFile -TotalCount 1
    if ($line) {
      return "$line".Trim()
    }
  }
  return ""
}

function Set-InstalledVersion {
  param([string]$InstallDir, [string]$Tag)

  Set-Content -LiteralPath (Get-VersionFilePath -InstallDir $InstallDir) -Value $Tag -Encoding UTF8
}

function Copy-ReleaseFiles {
  param([string]$SourceDir, [string]$InstallDir)

  New-Item -ItemType Directory -Path $InstallDir -Force | Out-Null
  $savedConfig = Save-ExistingConfig -InstallDir $InstallDir

  foreach ($name in @("internal", "web", "modernz")) {
    $target = Join-Path $InstallDir $name
    if (Test-Path -LiteralPath $target) {
      Remove-Item -LiteralPath $target -Recurse -Force
    }
  }

  foreach ($name in @("javboss.exe", "javboss", "javboss.command")) {
    $target = Join-Path $InstallDir $name
    if (Test-Path -LiteralPath $target) {
      Remove-Item -LiteralPath $target -Force
    }
  }

  Copy-Item -Path (Join-Path $SourceDir "*") -Destination $InstallDir -Recurse -Force
  Restore-ExistingConfig -SavedConfig $savedConfig -InstallDir $InstallDir
}

function Get-JavBossProcessInInstallDir {
  param([string]$InstallDir)

  $exe = Join-Path $InstallDir "javboss.exe"
  if (-not (Test-Path -LiteralPath $exe)) {
    return @()
  }

  $target = [System.IO.Path]::GetFullPath($exe)
  try {
    $processes = Get-CimInstance Win32_Process -Filter "Name = 'javboss.exe'"
  } catch {
    $processes = Get-WmiObject Win32_Process -Filter "Name = 'javboss.exe'"
  }

  @($processes | Where-Object {
    $_.ExecutablePath -and
      ([System.String]::Equals(
        [System.IO.Path]::GetFullPath($_.ExecutablePath),
        $target,
        [System.StringComparison]::OrdinalIgnoreCase
      ))
  })
}

function Assert-JavBossNotRunning {
  param([string]$InstallDir)

  $running = @(Get-JavBossProcessInInstallDir -InstallDir $InstallDir)
  if ($running.Count -eq 0) {
    return
  }

  $ids = ($running | ForEach-Object { $_.ProcessId }) -join ", "
  if (Test-PrefersChinese) {
    Fail "JavBoss 已经从 $InstallDir 启动（PID：$ids）。安装或升级前请先退出 JavBoss。"
  }
  Fail "JavBoss is already running from $InstallDir (pid: $ids). Please exit JavBoss before installing or upgrading."
}

function New-JavBossShortcut {
  param([string]$ShortcutPath, [string]$InstallDir)

  if (-not $ShortcutPath) {
    return
  }

  $target = Join-Path $InstallDir "javboss.exe"
  $shortcutDir = Split-Path -Parent $ShortcutPath
  if ($shortcutDir) {
    New-Item -ItemType Directory -Path $shortcutDir -Force | Out-Null
  }

  $shell = New-Object -ComObject WScript.Shell
  $shortcut = $shell.CreateShortcut($ShortcutPath)
  $shortcut.TargetPath = $target
  $shortcut.WorkingDirectory = $InstallDir
  $shortcut.IconLocation = "$target,0"
  $shortcut.Save()
  Write-Log "shortcut installed: $ShortcutPath"
}

function New-JavBossShortcuts {
  param([string]$InstallDir)

  $programs = [Environment]::GetFolderPath("Programs")
  if ($programs) {
    New-JavBossShortcut -ShortcutPath (Join-Path $programs "JavBoss.lnk") -InstallDir $InstallDir
  }

  $desktop = [Environment]::GetFolderPath("Desktop")
  if ($desktop) {
    New-JavBossShortcut -ShortcutPath (Join-Path $desktop "JavBoss.lnk") -InstallDir $InstallDir
  }
}

function Start-JavBoss {
  param([string]$InstallDir)
  $exe = Join-Path $InstallDir "javboss.exe"
  Push-Location $InstallDir
  try {
    & $exe
  } finally {
    Pop-Location
  }
}

if (-not $Dir) {
  $baseDir = $env:LOCALAPPDATA
  if (-not $baseDir) {
    $baseDir = Join-Path $HOME "AppData\Local"
  }
  $Dir = Join-Path $baseDir "JavBoss"
}
$Dir = [System.IO.Path]::GetFullPath($Dir)
Assert-JavBossNotRunning -InstallDir $Dir

$platform = Get-PlatformLabel
$tag = Normalize-Tag -InputVersion $Version -Repository $Repo
$fileName = "javboss-$tag-$platform.zip"
$url = "https://github.com/$Repo/releases/download/$tag/$fileName"

if ((Get-InstalledVersion -InstallDir $Dir) -eq $tag) {
  Write-Log "JavBoss $tag is already installed; skipping download"
  if (-not $NoStart) {
    Write-Log "starting JavBoss"
    Start-JavBoss -InstallDir $Dir
  } else {
    Write-Log "start later with: $(Join-Path $Dir "javboss.exe")"
  }
  return
}

$tempRoot = Join-Path ([System.IO.Path]::GetTempPath()) ("javboss-install-" + [System.Guid]::NewGuid().ToString())
$zipPath = Join-Path $tempRoot $fileName
$extractDir = Join-Path $tempRoot "extract"

try {
  New-Item -ItemType Directory -Path $extractDir -Force | Out-Null

  Write-Log "downloading $url"
  Download-FileWithProgress -Url $url -OutFile $zipPath

  Write-Log "extracting release package"
  Expand-Archive -LiteralPath $zipPath -DestinationPath $extractDir -Force
  $releaseDir = Get-ChildItem -LiteralPath $extractDir -Directory | Select-Object -First 1
  if (-not $releaseDir -or -not (Test-Path -LiteralPath (Join-Path $releaseDir.FullName "javboss.exe"))) {
    Fail "release package layout is invalid"
  }

  Write-Log "installing to $Dir"
  Copy-ReleaseFiles -SourceDir $releaseDir.FullName -InstallDir $Dir
  New-JavBossShortcuts -InstallDir $Dir
  Set-InstalledVersion -InstallDir $Dir -Tag $tag

  Write-Log "installed JavBoss $tag"
  if (-not $NoStart) {
    Write-Log "starting JavBoss"
    Start-JavBoss -InstallDir $Dir
  } else {
    Write-Log "start later with: $(Join-Path $Dir "javboss.exe")"
  }
} finally {
  Remove-Item -LiteralPath $tempRoot -Recurse -Force -ErrorAction SilentlyContinue
}
