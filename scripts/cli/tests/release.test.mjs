import assert from "node:assert/strict";
import { spawn, spawnSync } from "node:child_process";
import fs from "node:fs";
import fsp from "node:fs/promises";
import os from "node:os";
import path from "node:path";
import test from "node:test";
import vm from "node:vm";

// Exercise release orchestration with real directories and ZIP files, replacing
// expensive builds/downloads so the tests do not need platform binaries.
const source = fs.readFileSync(new URL("../cli.mjs", import.meta.url), "utf8")
  .replace(/^#!.*\n/, "")
  .replace(/^import .*;\n/gm, "")
  .split("\nmain().catch(")[0];
const hasZip = spawnSync("zip", ["-v"]).status === 0 &&
  spawnSync("unzip", ["-v"]).status === 0;

function releaseContext(t, goos, goarch = "amd64") {
  const root = fs.mkdtempSync(path.join(os.tmpdir(), "javboss-release-test-"));
  t.after(() => fs.rmSync(root, { recursive: true, force: true }));
  fs.writeFileSync(path.join(root, "go.mod"), "module test\n");
  const context = vm.createContext({
    fs, fsp, os, path, spawn,
    console: { log() {}, error() {} },
    process: { ...process, argv: [process.execPath, path.join(root, "cli.mjs")], exitCode: 0 },
    calls: [], goos, goarch,
  });
  vm.runInContext(source + `
    globalThis.choice = PLATFORM_CHOICES.find(p => p.goos === goos && p.goarch === goarch);
    globalThis.outDir = path.join(ROOT_DIR, 'release', 'javboss-test-' + choice.label);
    globalThis.zipPath = outDir + '.zip';
    globalThis.originalBuildBackendRelease = buildBackendRelease;
    isBundledFfprobeReady = async () => true;
    isBundledMpvReady = async () => true;
    buildWeb = async () => {};
    copyDir = async () => {};
    buildBackendRelease = async (_, dir, version) => {
      globalThis.builtVersion = version;
      await fsp.writeFile(path.join(dir, 'javboss'), 'server');
    };
    copyBundledMpv = async () => {};
    copyModernZAssets = async () => {};
    createReleaseConfig = async () => {};
    createMacCommandLauncher = async () => {};
    downloadFfprobe = async () => calls.push('download-ffprobe');
    downloadMpv = async () => calls.push('download-mpv');
    globalThis.cli = { runRelease, createZip, downloadDependencies, handleRelease };
  `, context);
  const probe = path.join(root, "bin", context.choice.label, goos === "windows" ? "ffprobe.exe" : "ffprobe");
  fs.mkdirSync(path.dirname(probe), { recursive: true });
  fs.writeFileSync(probe, "ffprobe");
  return context;
}

test("extension release includes private storage and loads it before the popup", { skip: !hasZip }, async (t) => {
  const ctx = releaseContext(t, "linux");
  const root = vm.runInContext("ROOT_DIR", ctx);
  await fsp.cp(new URL("../../../browser-extension", import.meta.url), path.join(root, "browser-extension"), { recursive: true });
  await vm.runInContext(`
    copyDir = (source, destination) => fsp.cp(source, destination, { recursive: true });
    runBrowserExtensionRelease();
  `, ctx);
  const manifest = JSON.parse(await fsp.readFile(path.join(root, "browser-extension", "manifest.json"), "utf8"));
  const name = `javboss-browser-extension-v${manifest.version}`;
  const archive = path.join(root, "release", `${name}.zip`);
  const entries = spawnSync("unzip", ["-Z1", archive], { encoding: "utf8" });
  assert.equal(entries.status, 0, entries.stderr);
  assert.ok(entries.stdout.split("\n").includes(`${name}/settings-storage.js`));
  const popup = spawnSync("unzip", ["-p", archive, `${name}/popup.html`], { encoding: "utf8" });
  assert.match(popup.stdout, /settings-storage\.js[\s\S]*popup\.js/);
  assert.ok(!entries.stdout.includes("/tests/"));
});

for (const goos of ["windows", "linux", "darwin"]) {
  test(`${goos} release ZIP uses JavBoss without nesting the build output`, { skip: !hasZip }, async (t) => {
    const ctx = releaseContext(t, goos);
    await ctx.cli.runRelease(ctx.choice, "test");
    assert.equal(ctx.process.exitCode, 0);
    assert.equal(await fsp.readFile(path.join(ctx.outDir, "javboss"), "utf8"), "server");
    assert.equal(fs.existsSync(path.join(ctx.outDir, "JavBoss")), false);
    const result = spawnSync("unzip", ["-Z1", ctx.zipPath], { encoding: "utf8" });
    assert.equal(result.status, 0, result.stderr);
    const entries = result.stdout.trim().split("\n");
    assert.ok(entries.includes("JavBoss/javboss"));
    assert.ok(entries.every((entry) => entry.startsWith("JavBoss/")));
    assert.match(path.basename(ctx.zipPath), /^javboss-test-.+\.zip$/);
  });

  test(`${goos} release selects the correct executable subsystem`, async (t) => {
    const ctx = releaseContext(t, goos);
    vm.runInContext(`
      commandExists = async () => true;
      runCommand = async (command, args, options) => calls.push({ command, args, options });
    `, ctx);
    await ctx.originalBuildBackendRelease(ctx.choice, ctx.outDir, "v1.2.3-rc.1");
    const { command, args, options } = ctx.calls[0];
    assert.equal(command, "go");
    assert.equal(args[0], "build");
    const ldflags = args[args.indexOf("-ldflags") + 1];
    assert.match(ldflags, /-X main\.buildMode=release/);
    assert.match(ldflags, /-X javboss\/internal\/common\.Version=v1\.2\.3-rc\.1(?:\s|$)/);
    assert.equal(ldflags.includes("-H windowsgui"), goos === "windows");
    assert.equal(options.env.GOOS, goos);
    assert.equal(options.env.CGO_ENABLED, "1");
    assert.equal(args[args.indexOf("-o") + 1], path.join(ctx.outDir, goos === "windows" ? "javboss.exe" : "javboss"));
  });
}

for (const [goos, goarch] of [["windows", "amd64"], ["linux", "amd64"], ["darwin", "amd64"], ["darwin", "arm64"]]) {
  test(`${goos}/${goarch} releases omit FFmpeg even when rebuilding an old ZIP`, { skip: !hasZip }, async (t) => {
    const ctx = releaseContext(t, goos, goarch);
    // Simulate an existing release with the formerly bundled FFmpeg.
    const oldBinary = path.join(ctx.outDir, "internal", "bin", goos === "windows" ? "ffmpeg.exe" : "ffmpeg");
    await fsp.mkdir(path.dirname(oldBinary), { recursive: true });
    await fsp.writeFile(oldBinary, "old ffmpeg");
    await ctx.cli.createZip(ctx.outDir, ctx.zipPath);
    await ctx.cli.handleRelease("test", `${goos}/${goarch}`);
    assert.equal(ctx.builtVersion, "test");
    assert.equal(ctx.process.exitCode, 0);
    const result = spawnSync("unzip", ["-Z1", ctx.zipPath], { encoding: "utf8" });
    assert.equal(result.status, 0, result.stderr);
    assert.match(result.stdout, /\/javboss\n/);
    assert.match(result.stdout, /\/internal\/bin\/ffprobe(?:\.exe)?\n/);
    assert.doesNotMatch(result.stdout, /\/ffmpeg(?:\.exe)?\n/);
    assert.ok(result.stdout.trim().split("\n").every((entry) => entry.startsWith("JavBoss/")));
  });

  test(`${goos}/${goarch} dependency downloads skip FFmpeg`, async (t) => {
    const ctx = releaseContext(t, goos, goarch);
    await ctx.cli.downloadDependencies(ctx.choice);
    assert.equal(ctx.calls.join(","), "download-ffprobe,download-mpv");
  });
}
