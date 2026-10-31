$ErrorActionPreference = "Stop"
$ProgressPreference = "SilentlyContinue"

$Repo = "Solr159/JavBoss"

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

function Get-LatestReleaseAsset {
  param([string]$Platform)

  try {
    $response = Invoke-WebRequest -Uri "https://github.com/$Repo/releases/latest" -Method Head -UseBasicParsing -MaximumRedirection 10 -Headers @{ "User-Agent" = "JavBoss-Installer" }
  } catch {
    Fail "could not resolve the latest release: $_"
  }

  # Windows PowerShell 5.1 and PowerShell 7 expose the final URL differently.
  if ($response.BaseResponse.ResponseUri) {
    $releaseUrl = $response.BaseResponse.ResponseUri.AbsoluteUri
  } else {
    $releaseUrl = $response.BaseResponse.RequestMessage.RequestUri.AbsoluteUri
  }
  $prefix = "https://github.com/$Repo/releases/tag/"
  if (-not $releaseUrl -or -not $releaseUrl.StartsWith($prefix, [System.StringComparison]::Ordinal)) {
    Fail "invalid latest release redirect: $releaseUrl"
  }
  $tag = $releaseUrl.Substring($prefix.Length)
  Compare-ReleaseVersions -Left $tag -Right $tag | Out-Null
  $fileName = "javboss-$tag-$Platform.zip"
  $url = "https://github.com/$Repo/releases/download/$tag/$fileName"
  return [PSCustomObject]@{ Tag = $tag; FileName = $fileName; Url = $url }
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
  return Join-Path $InstallDir ".version"
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

function Compare-VersionIdentifier {
  param([string]$Left, [string]$Right, [bool]$Numeric)

  if ($Numeric -and $Left.Length -ne $Right.Length) {
    return [Math]::Sign($Left.Length - $Right.Length)
  }
  return [Math]::Sign([String]::CompareOrdinal($Left, $Right))
}

function Compare-ReleaseVersions {
  param([string]$Left, [string]$Right)

  $pattern = '^v(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)(-[0-9A-Za-z-]+(\.[0-9A-Za-z-]+)*)?(\+[0-9A-Za-z-]+(\.[0-9A-Za-z-]+)*)?\z'
  foreach ($version in @($Left, $Right)) {
    if ($version -cnotmatch $pattern) {
      Fail "cannot compare invalid version '$version'; check the release tag and installed .version file"
    }
    $parts = ($version -split '\+', 2)[0] -split '-', 2
    if ($parts.Count -eq 2) {
      foreach ($identifier in ($parts[1] -split '\.')) {
        if ($identifier -match '^0[0-9]+$') { Fail "invalid numeric prerelease identifier in '$version'" }
      }
    }
  }

  # Build metadata does not affect precedence; compare prereleases after the core.
  $a = ($Left.Substring(1) -split '\+', 2)[0] -split '-', 2
  $b = ($Right.Substring(1) -split '\+', 2)[0] -split '-', 2
  $ac = $a[0] -split '\.'
  $bc = $b[0] -split '\.'
  for ($i = 0; $i -lt 3; $i++) {
    $comparison = Compare-VersionIdentifier $ac[$i] $bc[$i] $true
    if ($comparison -ne 0) { return $comparison }
  }
  if ($a.Count -eq 1 -or $b.Count -eq 1) {
    return [Math]::Sign($b.Count - $a.Count)
  }
  $ap = $a[1] -split '\.'
  $bp = $b[1] -split '\.'
  for ($i = 0; $i -lt [Math]::Min($ap.Count, $bp.Count); $i++) {
    $an = $ap[$i] -match '^[0-9]+$'
    $bn = $bp[$i] -match '^[0-9]+$'
    if ($an -ne $bn) {
      if ($an) { return -1 }
      return 1
    }
    $comparison = Compare-VersionIdentifier $ap[$i] $bp[$i] $an
    if ($comparison -ne 0) { return $comparison }
  }
  return [Math]::Sign($ap.Count - $bp.Count)
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

$baseDir = $env:LOCALAPPDATA
if (-not $baseDir) {
  $baseDir = Join-Path $HOME "AppData\Local"
}
$Dir = Join-Path $baseDir "JavBoss"
$Dir = [System.IO.Path]::GetFullPath($Dir)
$platform = Get-PlatformLabel
Write-Log "checking the latest JavBoss release"
$asset = Get-LatestReleaseAsset -Platform $platform
$tag = $asset.Tag
$fileName = $asset.FileName
$url = $asset.Url

if (Test-Path -LiteralPath (Join-Path $Dir "javboss.exe")) {
  $currentVersion = Get-InstalledVersion -InstallDir $Dir
  if ((Compare-ReleaseVersions -Left $tag -Right $currentVersion) -le 0) {
    Write-Log "JavBoss $currentVersion is already installed; latest is $tag, no update needed"
    return
  }
}

Assert-JavBossNotRunning -InstallDir $Dir

$tempRoot = Join-Path ([System.IO.Path]::GetTempPath()) ("javboss-install-" + [System.Guid]::NewGuid().ToString())
$zipPath = Join-Path $tempRoot $fileName
$extractDir = Join-Path $tempRoot "extract"

try {
  New-Item -ItemType Directory -Path $extractDir -Force | Out-Null

  Write-Log "downloading $url"
  Invoke-WebRequest -Uri $url -OutFile $zipPath -Headers @{ "User-Agent" = "JavBoss-Installer" }

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
  Write-Log "starting JavBoss"
  Start-JavBoss -InstallDir $Dir
} finally {
  Remove-Item -LiteralPath $tempRoot -Recurse -Force -ErrorAction SilentlyContinue
}
