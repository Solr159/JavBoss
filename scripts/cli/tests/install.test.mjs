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
const psSource = fs.readFileSync(new URL("scripts/install.ps1", root), "utf8")
  .split("\n$baseDir =")[0];
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

function runInstaller(dir, tool, { installedTag = "v9.8.6", redirect = releaseUrl, queryFails = false, downloadFails = false } = {}) {
  const installDir = path.join(dir, "installed");
  fs.mkdirSync(path.join(installDir, "data"), { recursive: true });
  fs.writeFileSync(path.join(installDir, "data", "keep.db"), "user database");
  fs.writeFileSync(path.join(installDir, "config.toml"), "user config");
  if (installedTag) {
    fs.writeFileSync(path.join(installDir, "javboss"), "old binary");
    fs.writeFileSync(path.join(installDir, ".version"), installedTag + "\n");
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
  for (const installedTag of [null, "v9.8.6"]) {
    test(`${tool} ${installedTag ? "upgrade" : "install"} follows latest and records its real tag`, { skip: !hasShellTools }, (t) => {
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

  test(`${tool} skips the archive download when latest is already installed`, { skip: !hasShellTools }, (t) => {
    const dir = fixture(t);
    const result = runInstaller(dir, tool, { installedTag: "v9.8.7" });
    assert.equal(result.status, 0, result.stderr);
    assert.match(result.stdout, /already installed/);
    assert.equal(fs.readFileSync(path.join(dir, "installed", "javboss"), "utf8"), "old binary");
    assert.equal(fs.readFileSync(path.join(dir, "requests"), "utf8").trim(), latestUrl);
  });

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

test("version update only changes README links", (t) => {
  const dir = fixture(t);
  fs.mkdirSync(path.join(dir, "scripts"));
  fs.copyFileSync(new URL("scripts/update-release-version.mjs", root), path.join(dir, "scripts", "update-release-version.mjs"));
  for (const name of ["install.sh", "install.ps1"]) {
    fs.writeFileSync(path.join(dir, "scripts", name), "installer must stay unchanged\n");
  }
  fs.writeFileSync(path.join(dir, "README.md"), "https://github.com/Solr159/JavBoss/releases/download/v1.0.0/javboss-v1.0.0-linux-x86_64.zip\n");
  const result = spawnSync(process.execPath, [path.join(dir, "scripts", "update-release-version.mjs"), "v9.8.7"], { encoding: "utf8" });
  assert.equal(result.status, 0, result.stderr);
  assert.match(fs.readFileSync(path.join(dir, "README.md"), "utf8"), /download\/v9\.8\.7\/javboss-v9\.8\.7-linux-x86_64.zip/);
  for (const name of ["install.sh", "install.ps1"]) {
    assert.equal(fs.readFileSync(path.join(dir, "scripts", name), "utf8"), "installer must stay unchanged\n");
  }
});
