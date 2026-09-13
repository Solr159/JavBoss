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

function releaseContext(t, goos) {
  const root = fs.mkdtempSync(path.join(os.tmpdir(), "javboss-release-test-"));
  t.after(() => fs.rmSync(root, { recursive: true, force: true }));
  fs.writeFileSync(path.join(root, "go.mod"), "module test\n");
  const context = vm.createContext({
    fs, fsp, os, path, spawn,
    console: { log() {}, error() {} },
    process: { ...process, argv: [process.execPath, path.join(root, "cli.mjs")], exitCode: 0 },
    calls: [], goos,
  });
  vm.runInContext(source + `
    globalThis.choice = PLATFORM_CHOICES.find(p => p.goos === goos);
    globalThis.outDir = path.join(ROOT_DIR, 'release', 'javboss-test-' + choice.label);
    globalThis.zipPath = outDir + '.zip';
    isBundledFfprobeReady = async () => true;
    isBundledMpvReady = async () => true;
    isBundledFfmpegReady = async () => { calls.push('check-ffmpeg'); return true; };
    buildWeb = async () => {};
    copyDir = async () => {};
    buildBackendRelease = async (_, dir) => fsp.writeFile(path.join(dir, 'javboss'), 'server');
    copyBundledFfprobe = async () => {};
    copyBundledFfmpeg = async (_, dir) => {
      calls.push('bundle-ffmpeg');
      await fsp.writeFile(path.join(dir, ffmpegBinName(goos)), 'ffmpeg');
    };
    copyBundledMpv = async () => {};
    copyModernZAssets = async () => {};
    createReleaseConfig = async () => {};
    createMacCommandLauncher = async () => {};
    downloadFfprobe = async () => calls.push('download-ffprobe');
    downloadFfmpeg = async () => calls.push('download-ffmpeg');
    downloadMpv = async () => calls.push('download-mpv');
    globalThis.cli = { runRelease, createZip, downloadDependencies, handleRelease };
  `, context);
  return context;
}

for (const goos of ["windows", "linux"]) {
  test(`${goos} releases omit FFmpeg even when rebuilding an old ZIP`, { skip: !hasZip }, async (t) => {
    const ctx = releaseContext(t, goos);
    // Simulate an existing release with the formerly bundled FFmpeg.
    const oldBinary = path.join(ctx.outDir, "internal", "bin", goos === "windows" ? "ffmpeg.exe" : "ffmpeg");
    await fsp.mkdir(path.dirname(oldBinary), { recursive: true });
    await fsp.writeFile(oldBinary, "old ffmpeg");
    await ctx.cli.createZip(ctx.outDir, ctx.zipPath);
    await ctx.cli.handleRelease("test", `${goos}/amd64`);
    assert.equal(ctx.process.exitCode, 0);
    assert.equal(ctx.calls.length, 0, "must neither check nor bundle FFmpeg");
    const result = spawnSync("unzip", ["-Z1", ctx.zipPath], { encoding: "utf8" });
    assert.equal(result.status, 0, result.stderr);
    assert.match(result.stdout, /\/javboss\n/);
    assert.doesNotMatch(result.stdout, /\/ffmpeg(?:\.exe)?\n/);
  });

  test(`${goos} dependency downloads skip FFmpeg`, async (t) => {
    const ctx = releaseContext(t, goos);
    await ctx.cli.downloadDependencies(ctx.choice);
    assert.equal(ctx.calls.join(","), "download-ffprobe,download-mpv");
  });
}

test("macOS still downloads and bundles FFmpeg", { skip: !hasZip }, async (t) => {
  const ctx = releaseContext(t, "darwin");
  await ctx.cli.downloadDependencies(ctx.choice);
  await ctx.cli.runRelease(ctx.choice, "test");
  assert.equal(ctx.process.exitCode, 0);
  assert.equal(ctx.calls.join(","), "download-ffprobe,download-ffmpeg,download-mpv,check-ffmpeg,bundle-ffmpeg");
  assert.equal(await fsp.readFile(path.join(ctx.outDir, "ffmpeg"), "utf8"), "ffmpeg");
});
