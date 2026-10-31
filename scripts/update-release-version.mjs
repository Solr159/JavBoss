#!/usr/bin/env node
import fs from "node:fs/promises";
import path from "node:path";
import { fileURLToPath } from "node:url";

const SCRIPT_DIR = path.dirname(fileURLToPath(import.meta.url));
const ROOT_DIR = path.dirname(SCRIPT_DIR);
const REPO = "Solr159/JavBoss";
const version = String(process.argv[2] || "").trim();
const metadataPath = process.argv[3];
// The workflow reads its script from the selected ref, but edits only main's README.
const readmePath = path.resolve(process.argv[4] || path.join(ROOT_DIR, "README.md"));
const applicationRelease = /^v(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$/.test(version);
const extensionRelease = /^browser-extension-v[0-9]+(?:\.[0-9]+){0,3}$/.test(version);
if (!applicationRelease && !extensionRelease) {
  console.error("usage: node scripts/update-release-version.mjs RELEASE_TAG [RELEASE_JSON] [README_PATH]");
  process.exit(1);
}

const extensionAssetPattern = /^javboss-browser-extension-v([0-9]+(?:\.[0-9]+){0,3})\.zip$/;
let extensionAsset;
if (metadataPath) {
  const release = JSON.parse(await fs.readFile(metadataPath, "utf8"));
  const extensions = release.assets.filter(asset => extensionAssetPattern.test(asset.name));
  if (extensions.length > 1) throw new Error("release has multiple browser extension packages");
  extensionAsset = extensions[0]?.name;
} else if (extensionRelease) {
  extensionAsset = `javboss-${version}.zip`;
}
if (extensionRelease && extensionAsset !== `javboss-${version}.zip`) {
  throw new Error("release is missing its browser extension package");
}

const before = await fs.readFile(readmePath, "utf8");
let content = before;
function replaceRequired(pattern, replacement) {
  if (!pattern.test(content)) throw new Error(`README.md: pattern not found: ${pattern}`);
  pattern.lastIndex = 0;
  content = content.replace(pattern, replacement);
}

if (applicationRelease) {
  const platformPattern = "(windows-x86_64|linux-x86_64|macos-x86_64|macos-arm64)";
  const existingVersionPattern = "v[0-9]+\\.[0-9]+\\.[0-9]+(?:[-.][0-9A-Za-z.-]+)?";
  const releaseUrlPattern = new RegExp(
    `https://github\\.com/${REPO}/releases/download/` +
      `${existingVersionPattern}/javboss-${existingVersionPattern}-${platformPattern}\\.zip`, "g",
  );
  replaceRequired(releaseUrlPattern,
    `https://github.com/${REPO}/releases/download/${version}/javboss-${version}-$1.zip`);
}

// Compare the extension's own 1–4 numeric version components, not the main tag.
function compareExtensionVersions(left, right) {
  const a = left.split(".").map(BigInt);
  const b = right.split(".").map(BigInt);
  for (let i = 0; i < 4; i++) {
    const diff = (a[i] ?? 0n) - (b[i] ?? 0n);
    if (diff !== 0n) return diff > 0n ? 1 : -1;
  }
  return 0;
}

if (extensionAsset) {
  const extensionVersion = extensionAsset.match(extensionAssetPattern)[1];
  const extensionUrlPattern = new RegExp(
    `https://github\\.com/${REPO}/releases/download/[^/\\s<>"()]+/` +
      "javboss-browser-extension-v([0-9]+(?:\\.[0-9]+){0,3})\\.zip", "g",
  );
  replaceRequired(extensionUrlPattern, (url, installedVersion) => {
    if (compareExtensionVersions(extensionVersion, installedVersion) < 0) return url;
    return `https://github.com/${REPO}/releases/download/${version}/${extensionAsset}`;
  });
}

if (content !== before) await fs.writeFile(readmePath, content);
console.log(content === before ? "README download links are already up to date" : `updated README download links for ${version}`);
