#!/usr/bin/env node
import fs from "node:fs/promises";
import path from "node:path";
import { fileURLToPath } from "node:url";

const SCRIPT_DIR = path.dirname(fileURLToPath(import.meta.url));
const ROOT_DIR = path.dirname(SCRIPT_DIR);
const REPO = "Solr159/JavBoss";

const version = String(process.argv[2] || "").trim();
if (!/^v[0-9]+\.[0-9]+\.[0-9]+([-.][0-9A-Za-z.-]+)?$/.test(version)) {
  console.error("usage: node scripts/update-release-version.mjs vX.Y.Z");
  process.exit(1);
}

async function replaceRequired(relativePath, replacements) {
  const filePath = path.join(ROOT_DIR, relativePath);
  let content = await fs.readFile(filePath, "utf8");
  let changed = false;

  for (const [pattern, replacement] of replacements) {
    pattern.lastIndex = 0;
    if (!pattern.test(content)) {
      throw new Error(`${relativePath}: pattern not found: ${pattern}`);
    }
    pattern.lastIndex = 0;
    content = content.replace(pattern, replacement);
    changed = true;
  }

  if (changed) {
    await fs.writeFile(filePath, content);
  }
}

const platformPattern = "(windows-x86_64|linux-x86_64|macos-x86_64|macos-arm64)";
const existingVersionPattern = "v[0-9]+\\.[0-9]+\\.[0-9]+(?:[-.][0-9A-Za-z.-]+)?";
const releaseUrlPattern = new RegExp(
  `https://github\\.com/${REPO}/releases/download/` +
    `${existingVersionPattern}/` +
    `javboss-${existingVersionPattern}-` +
    `${platformPattern}\\.zip`,
  "g",
);

const releaseUrlReplacement = `https://github.com/${REPO}/releases/download/${version}/javboss-${version}-$1.zip`;

await replaceRequired("scripts/install.sh", [
  [/^VERSION="[^"]+"/m, `VERSION="${version}"`],
]);

await replaceRequired("scripts/install.ps1", [
  [/^\$Version = "[^"]+"/m, `$Version = "${version}"`],
]);

await replaceRequired("README.md", [[releaseUrlPattern, releaseUrlReplacement]]);
await replaceRequired("README.en.md", [[releaseUrlPattern, releaseUrlReplacement]]);

console.log(`updated release version to ${version}`);
