import assert from "node:assert/strict";
import { spawnSync } from "node:child_process";
import fs from "node:fs";
import os from "node:os";
import path from "node:path";
import test from "node:test";

const repo = "Solr159/JavBoss";
const root = new URL("../../../", import.meta.url);
const shellSource = fs.readFileSync(new URL("scripts/install.sh", root), "utf8")
  .replace(/\nmain "\$@"\s*$/, "\n");
const psFullSource = fs.readFileSync(new URL("scripts/install.ps1", root), "utf8");
const psSource = psFullSource.split("\n$baseDir =")[0];
const hasShellTools = ["bash", "zip", "unzip"].every(
  (command) => spawnSync("bash", ["-c", 'command -v "$1"', "bash", command]).status === 0,
);
const hasPowerShell = spawnSync("pwsh", ["-NoProfile", "-Command", "exit 0"]).status === 0;

function fixture(t) {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), "javboss-install-test-"));
  t.after(() => fs.rmSync(dir, { recursive: true, force: true }));
  return dir;
}

const latestUrl = `https://github.com/${repo}/releases/latest`;
const releaseUrl = `https://github.com/${repo}/releases/tag/v9.8.7`;
const downloadUrl = `https://github.com/${repo}/releases/download/v9.8.7/javboss-v9.8.7-linux-x86_64.zip`;

function runInstaller(dir, tool, { installedTag = "v9.8.6", missingVersion = false, redirect = releaseUrl, queryFails = false, downloadFails = false } = {}) {
  const installDir = path.join(dir, "installed");
  fs.mkdirSync(path.join(installDir, "data"), { recursive: true });
  fs.writeFileSync(path.join(installDir, "data", "keep.db"), "user database");
  fs.writeFileSync(path.join(installDir, "config.toml"), "user config");
  if (installedTag) {
    fs.writeFileSync(path.join(installDir, "javboss"), "old binary");
    if (!missingVersion) fs.writeFileSync(path.join(installDir, ".version"), installedTag + "\n");
  }
  const packageDir = path.join(dir, "package");
  fs.mkdirSync(packageDir);
  fs.writeFileSync(path.join(packageDir, "javboss"), "#!/bin/sh\nprintf started > started\n", { mode: 0o755 });
  fs.writeFileSync(path.join(packageDir, "config.toml"), "default config");
  const zip = spawnSync("zip", ["-qr", path.join(dir, "release.zip"), "package"], { cwd: dir, encoding: "utf8" });
  assert.equal(zip.status, 0, zip.stderr);

  // Exercise both download clients, extraction and version tracking without
  // network requests or writes outside the disposable installation directory.
  const script = shellSource + `
detect_platform() { printf linux-x86_64; }
default_install_dir() { printf '%s/installed' "$TEST_ROOT"; }
running_javboss_pids() { return 0; }
command_exists() {
  case "$1" in
    curl|wget) [[ "$1" == "$DOWNLOAD_TOOL" ]] ;;
    jq) return 1 ;;
    *) command -v "$1" >/dev/null 2>&1 ;;
  esac
}
mock_client() {
  local request_url="" dest="" head=0
  while [[ "$#" -gt 0 ]]; do
    case "$1" in
      --head|--spider) head=1 ;;
      -o|-O) shift; dest="$1" ;;
      https://*) request_url="$1" ;;
    esac
    shift
  done
  printf '%s\\n' "$request_url" >> "$TEST_ROOT/requests"
  if [[ "$head" == 1 ]]; then
    [[ "$request_url" == "https://github.com/Solr159/JavBoss/releases/latest" ]] || return 99
    [[ "$QUERY_FAILS" != 1 ]] || return 1
    if [[ "$DOWNLOAD_TOOL" == curl ]]; then
      printf '%s' "$REDIRECT_URL"
    else
      printf '  HTTP/1.1 302 Found\\r\\n  Location: /Solr159/JavBoss/releases/latest\\r\\n  HTTP/1.1 302 Found\\r\\n  location: %s\\r\\n  HTTP/1.1 200 OK\\r\\n' "$REDIRECT_URL" >&2
    fi
  else
    [[ "$request_url" == "https://github.com/Solr159/JavBoss/releases/download/v9.8.7/javboss-v9.8.7-linux-x86_64.zip" ]] || return 99
    [[ "$DOWNLOAD_FAILS" != 1 ]] || return 1
    cp "$TEST_ROOT/release.zip" "$dest"
  fi
}
curl() { mock_client "$@"; }
wget() { mock_client "$@"; }
main
`;
  return spawnSync("bash", ["-c", script], {
    encoding: "utf8",
    env: {
      ...process.env, TEST_ROOT: dir, DOWNLOAD_TOOL: tool, REDIRECT_URL: redirect,
      QUERY_FAILS: queryFails ? "1" : "0", DOWNLOAD_FAILS: downloadFails ? "1" : "0",
      JAVBOSS_LINK_DIR: path.join(dir, "bin"), JAVBOSS_PROFILE: path.join(dir, "profile"),
    },
  });
}

const invalidRedirects = [
  ["missing redirect", latestUrl],
  ["empty redirect", ""],
  ["extension release", `https://github.com/${repo}/releases/tag/browser-extension-v1.0.0`],
  ["foreign repository", "https://github.com/other/repo/releases/tag/v9.8.7"],
  ["invalid tag", `${releaseUrl}/extra`],
];

for (const tool of ["curl", "wget"]) {
  for (const installedTag of [null, "v9.8.6", "v9.7.99", "v9.8.7-rc.10"]) {
    test(`${tool} ${installedTag ? `upgrade from ${installedTag}` : "install"} follows latest and records its real tag`, { skip: !hasShellTools }, (t) => {
      const dir = fixture(t);
      const result = runInstaller(dir, tool, { installedTag });
      assert.equal(result.status, 0, result.stderr);
      const read = (name) => fs.readFileSync(path.join(dir, "installed", name), "utf8");
      assert.equal(read(".version"), "v9.8.7\n");
      assert.equal(read("config.toml"), "user config");
      assert.equal(read("data/keep.db"), "user database");
      assert.equal(read("started"), "started");
      assert.deepEqual(fs.readFileSync(path.join(dir, "requests"), "utf8").trim().split("\n"), [latestUrl, downloadUrl]);
    });
  }

  for (const installedTag of ["v9.8.7", "v9.8.7+local.1", "v9.8.10", "v9.10.0", "v10.0.0-rc.1"]) {
    test(`${tool} skips the archive download with installed ${installedTag}`, { skip: !hasShellTools }, (t) => {
      const dir = fixture(t);
      const result = runInstaller(dir, tool, { installedTag });
      assert.equal(result.status, 0, result.stderr);
      assert.match(result.stdout, /already installed/);
      assert.equal(fs.readFileSync(path.join(dir, "installed", "javboss"), "utf8"), "old binary");
      assert.equal(fs.readFileSync(path.join(dir, "installed", ".version"), "utf8"), installedTag + "\n");
      assert.equal(fs.readFileSync(path.join(dir, "requests"), "utf8").trim(), latestUrl);
    });
  }

  for (const missingVersion of [false, true]) {
    test(`${tool} leaves an installation with ${missingVersion ? "missing" : "invalid"} version untouched`, { skip: !hasShellTools }, (t) => {
      const dir = fixture(t);
      const result = runInstaller(dir, tool, { installedTag: "unknown", missingVersion });
      assert.notEqual(result.status, 0);
      assert.match(result.stderr, /cannot compare installed version/);
      assert.equal(fs.readFileSync(path.join(dir, "installed", "javboss"), "utf8"), "old binary");
      assert.equal(fs.readFileSync(path.join(dir, "requests"), "utf8").trim(), latestUrl);
    });
  }

  const failures = [
    ...invalidRedirects.map(([name, redirect]) => [name, { redirect }]),
    ["latest request failure", { queryFails: true }],
    ["missing platform archive", { downloadFails: true }],
  ];
  for (const [name, options] of failures) {
    test(`${tool} rejects ${name} without replacing installed files`, { skip: !hasShellTools }, (t) => {
      const dir = fixture(t);
      const result = runInstaller(dir, tool, options);
      assert.notEqual(result.status, 0);
      assert.equal(fs.readFileSync(path.join(dir, "installed", "javboss"), "utf8"), "old binary");
      assert.equal(fs.readFileSync(path.join(dir, "installed", ".version"), "utf8"), "v9.8.6\n");
      assert.equal(fs.readFileSync(path.join(dir, "requests"), "utf8").trim().split("\n").length, options.downloadFails ? 2 : 1);
    });
  }
}

test("wget accepts origin-relative redirects", { skip: !hasShellTools }, (t) => {
  const result = runInstaller(fixture(t), "wget", { redirect: `/${repo}/releases/tag/v9.8.7` });
  assert.equal(result.status, 0, result.stderr);
});

for (const responseType of ["ResponseUri", "RequestMessage"]) {
  test(`PowerShell resolves latest using ${responseType} and rejects invalid redirects`, { skip: !hasPowerShell }, (t) => {
    const dir = fixture(t);
    const script = path.join(dir, "test.ps1");
    fs.writeFileSync(script, psSource + `
function Invoke-WebRequest {
  param($Uri, $Method, [switch]$UseBasicParsing, $MaximumRedirection, $Headers)
  if ($Uri -ne 'https://github.com/Solr159/JavBoss/releases/latest' -or $Method -ne 'Head' -or -not $UseBasicParsing -or $MaximumRedirection -ne 10) { throw 'wrong latest request' }
  if ($env:QUERY_FAILS -eq '1') { throw 'request failed' }
  if ($env:RESPONSE_TYPE -eq 'ResponseUri') {
    return [PSCustomObject]@{ BaseResponse = [PSCustomObject]@{ ResponseUri = [uri]$env:REDIRECT_URL } }
  }
  return [PSCustomObject]@{ BaseResponse = [PSCustomObject]@{ RequestMessage = [PSCustomObject]@{ RequestUri = [uri]$env:REDIRECT_URL } } }
}
$asset = Get-LatestReleaseAsset -Platform 'windows-x86_64'
if ($asset.Tag -ne 'v9.8.7' -or $asset.FileName -ne 'javboss-v9.8.7-windows-x86_64.zip' -or
    $asset.Url -ne 'https://github.com/Solr159/JavBoss/releases/download/v9.8.7/javboss-v9.8.7-windows-x86_64.zip') { throw 'wrong asset' }
`);
    const run = (redirect = releaseUrl, queryFails = false) => spawnSync("pwsh", ["-NoProfile", "-File", script], {
      encoding: "utf8", env: { ...process.env, RESPONSE_TYPE: responseType, REDIRECT_URL: redirect, QUERY_FAILS: queryFails ? "1" : "0" },
    });
    const result = run();
    assert.equal(result.status, 0, result.stderr);
    for (const [name, redirect] of invalidRedirects) {
      assert.notEqual(run(redirect).status, 0, name);
    }
    assert.notEqual(run(releaseUrl, true).status, 0, "latest request failure");
  });
}

const versionCases = [
  ["v2.1.3", "v2.1.3", 0],
  ["v2.1.4", "v2.1.3", 1],
  ["v2.1.2", "v2.1.3", -1],
  ["v2.10.0", "v2.9.99", 1],
  ["v2.1.10", "v2.1.9", 1],
  ["v10.0.0", "v9.99.99", 1],
  ["v2.0.0", "v3.0.0-rc.1", -1],
  ["v2.1.3", "v2.1.3-rc.1", 1],
  ["v2.1.3-rc.1", "v2.1.3", -1],
  ["v2.1.3-rc.10", "v2.1.3-rc.9", 1],
  ["v2.1.3-alpha", "v2.1.3-alpha.1", -1],
  ["v2.1.3-alpha.1", "v2.1.3-alpha.beta", -1],
  ["v2.1.3-beta", "v2.1.3-alpha", 1],
  ["v2.1.3-RC", "v2.1.3-rc", -1],
  ["v2.1.3+build.1", "v2.1.3+build.2", 0],
  ["v2.1.3-rc.1+build.1", "v2.1.3-rc.1", 0],
  ["v9007199254740993.0.0", "v9007199254740992.0.0", 1],
];
const invalidVersions = ["", "unknown", "v2.1", "v2.1.3.4", "v02.1.3", "v2.1.3-rc.01", "v2.1.3-", "v2.1.3+", "v2.1.3\n"];

test("Bash compares numeric versions, prereleases and build metadata", { skip: !hasShellTools }, (t) => {
  const dir = fixture(t);
  const cases = path.join(dir, "versions.tsv");
  fs.writeFileSync(cases, versionCases.map((row) => row.join("\t")).join("\n") + "\n");
  const result = spawnSync("bash", ["-c", shellSource + `
while IFS=$'\\t' read -r left right expected; do
  actual="$(compare_versions "$left" "$right")"
  [[ "$actual" == "$expected" ]] || { printf '%s vs %s: expected %s, got %s\\n' "$left" "$right" "$expected" "$actual" >&2; exit 1; }
done < "$TEST_CASES"
`], { encoding: "utf8", env: { ...process.env, TEST_CASES: cases } });
  assert.equal(result.status, 0, result.stderr);
  for (const version of invalidVersions) {
    const invalid = spawnSync("bash", ["-c", shellSource + '\ncompare_versions "$INVALID_VERSION" v2.1.3'], {
      encoding: "utf8", env: { ...process.env, INVALID_VERSION: version },
    });
    assert.notEqual(invalid.status, 0, JSON.stringify(version));
  }
});

test("PowerShell uses the same version precedence as Bash", { skip: !hasPowerShell }, (t) => {
  const dir = fixture(t);
  const script = path.join(dir, "compare.ps1");
  fs.writeFileSync(path.join(dir, "cases.json"), JSON.stringify({ cases: versionCases, invalid: invalidVersions }));
  fs.writeFileSync(script, psSource + `
$data = Get-Content -Raw (Join-Path $env:TEST_ROOT 'cases.json') | ConvertFrom-Json
foreach ($case in $data.cases) {
  $actual = Compare-ReleaseVersions $case[0] $case[1]
  if ($actual -ne $case[2]) { throw "wrong precedence for $case : $actual" }
}
function Fail { param($Message); throw $Message }
foreach ($version in $data.invalid) {
  $rejected = $false
  try { Compare-ReleaseVersions $version 'v2.1.3' | Out-Null } catch { $rejected = $true }
  if (-not $rejected) { throw "accepted invalid version: $version" }
}
`);
  const result = spawnSync("pwsh", ["-NoProfile", "-File", script], { encoding: "utf8", env: { ...process.env, TEST_ROOT: dir } });
  assert.equal(result.status, 0, result.stderr);
});

test("PowerShell only reaches the download step for a newer latest version", { skip: !hasPowerShell }, (t) => {
  const dir = fixture(t);
  const installDir = path.join(dir, "JavBoss");
  fs.mkdirSync(installDir);
  fs.writeFileSync(path.join(installDir, "javboss.exe"), "old binary");
  const script = path.join(dir, "install.ps1");
  fs.writeFileSync(script, psSource + `
function Get-PlatformLabel { return 'windows-x86_64' }
function Get-LatestReleaseAsset { return [PSCustomObject]@{ Tag = 'v9.8.7'; FileName = 'release.zip'; Url = 'https://example.invalid/release.zip' } }
function Assert-JavBossNotRunning {}
function Invoke-WebRequest { Set-Content (Join-Path $env:LOCALAPPDATA 'download-attempted') 'yes'; throw 'stop before download' }
` + psFullSource.slice(psFullSource.indexOf("\n$baseDir =")));
  for (const installed of ["v9.8.7", "v9.8.10", "v10.0.0", "unknown", "", "v9.7.99", "v9.8.7-rc.1"]) {
    const versionFile = path.join(installDir, ".version");
    if (installed) fs.writeFileSync(versionFile, installed + "\n");
    else fs.rmSync(versionFile, { force: true });
    const marker = path.join(dir, "download-attempted");
    fs.rmSync(marker, { force: true });
    const result = spawnSync("pwsh", ["-NoProfile", "-File", script], { encoding: "utf8", env: { ...process.env, LOCALAPPDATA: dir } });
    const shouldUpdate = ["v9.7.99", "v9.8.7-rc.1"].includes(installed);
    assert.equal(fs.existsSync(marker), shouldUpdate, installed + result.stderr);
    if (!["unknown", ""].includes(installed) && !shouldUpdate) assert.equal(result.status, 0, result.stderr);
    else assert.notEqual(result.status, 0);
    assert.equal(fs.readFileSync(path.join(installDir, "javboss.exe"), "utf8"), "old binary");
    if (installed) assert.equal(fs.readFileSync(versionFile, "utf8"), installed + "\n");
  }
});

test("version update only changes README links", (t) => {
  const dir = fixture(t);
  fs.mkdirSync(path.join(dir, "scripts"));
  fs.copyFileSync(new URL("scripts/update-release-version.mjs", root), path.join(dir, "scripts", "update-release-version.mjs"));
  for (const name of ["install.sh", "install.ps1"]) {
    fs.writeFileSync(path.join(dir, "scripts", name), "installer must stay unchanged\n");
  }
  const platforms = ["windows-x86_64", "linux-x86_64", "macos-x86_64", "macos-arm64"];
  const extension = "https://github.com/Solr159/JavBoss/releases/download/v1.0.0/javboss-browser-extension-v0.14.1.zip";
  const before = ["Release documentation", ...platforms.map(platform =>
    `https://github.com/Solr159/JavBoss/releases/download/v1.0.0/javboss-v1.0.0-${platform}.zip`), extension].join("\n");
  fs.writeFileSync(path.join(dir, "README.md"), before);
  const result = spawnSync(process.execPath, [path.join(dir, "scripts", "update-release-version.mjs"), "v9.8.7"], { encoding: "utf8" });
  assert.equal(result.status, 0, result.stderr);
  const expected = before.replaceAll(/download\/v1\.0\.0\/javboss-v1\.0\.0-/g, "download/v9.8.7/javboss-v9.8.7-");
  assert.equal(fs.readFileSync(path.join(dir, "README.md"), "utf8"), expected);
  const retry = spawnSync(process.execPath, [path.join(dir, "scripts", "update-release-version.mjs"), "v9.8.7"], { encoding: "utf8" });
  assert.equal(retry.status, 0, retry.stderr);
  assert.equal(fs.readFileSync(path.join(dir, "README.md"), "utf8"), expected);
  for (const name of ["install.sh", "install.ps1"]) {
    assert.equal(fs.readFileSync(path.join(dir, "scripts", name), "utf8"), "installer must stay unchanged\n");
  }
});
