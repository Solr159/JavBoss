import assert from "node:assert/strict";
import { spawnSync } from "node:child_process";
import fs from "node:fs";
import os from "node:os";
import path from "node:path";
import { fileURLToPath } from "node:url";
import test from "node:test";

const script = fileURLToPath(new URL("../../update-release-version.mjs", import.meta.url));
const baseURL = "https://github.com/Solr159/JavBoss/releases/download/";
const appURL = baseURL + "v1.0.0/javboss-v1.0.0-linux-x86_64.zip";
const extURL = version => baseURL + `v1.0.0/javboss-browser-extension-v${version}.zip`;

for (const scenario of [
  { name: "standalone extension updates only its own link", tag: "browser-extension-v0.15.0", extension: "0.15.0", current: "0.14.1", update: true },
  { name: "bundled extension uses the main release tag and its own asset version", tag: "v2.1.4", extension: "0.15.0", current: "0.14.1", update: true },
  { name: "main release without an extension preserves the extension link", tag: "v2.1.4", current: "0.14.1", update: false },
  { name: "old standalone retry cannot downgrade the extension", tag: "browser-extension-v0.14.0", extension: "0.14.0", current: "0.15.0", update: false },
  { name: "older bundled extension cannot downgrade a newer standalone link", tag: "v2.1.4", extension: "0.14.0", current: "0.15.0", update: false, standalone: true },
  { name: "extension versions compare numerically", tag: "browser-extension-v0.10.0", extension: "0.10.0", current: "0.9.0", update: true },
  { name: "extension versions support four components", tag: "browser-extension-v0.14.1.2", extension: "0.14.1.2", current: "0.14.1", update: true },
]) {
  test(scenario.name, (t) => {
    const dir = fs.mkdtempSync(path.join(os.tmpdir(), "javboss-readme-update-"));
    t.after(() => fs.rmSync(dir, { recursive: true, force: true }));
    const readme = path.join(dir, "README.md");
    const metadata = path.join(dir, "release.json");
    const oldExtension = scenario.standalone
      ? baseURL + `browser-extension-v${scenario.current}/javboss-browser-extension-v${scenario.current}.zip`
      : extURL(scenario.current);
    const before = `Keep documentation\n[App](${appURL})\n[Extension](${oldExtension})\n`;
    fs.writeFileSync(readme, before);
    fs.writeFileSync(metadata, JSON.stringify({ assets: scenario.extension ? [{ name: `javboss-browser-extension-v${scenario.extension}.zip` }] : [] }));
    let expected = before;
    if (scenario.tag.startsWith("v")) {
      expected = expected.replace(appURL, baseURL + `${scenario.tag}/javboss-${scenario.tag}-linux-x86_64.zip`);
    }
    if (scenario.update) {
      expected = expected.replace(oldExtension, baseURL + `${scenario.tag}/javboss-browser-extension-v${scenario.extension}.zip`);
    }
    for (let retry = 0; retry < 2; retry++) {
      const result = spawnSync(process.execPath, [script, scenario.tag, metadata, readme], { encoding: "utf8" });
      assert.equal(result.status, 0, result.stderr);
      assert.equal(fs.readFileSync(readme, "utf8"), expected);
    }
  });
}

for (const scenario of [
  { name: "missing extension asset", tag: "browser-extension-v0.15.0", assets: [] },
  { name: "mismatched extension asset", tag: "browser-extension-v0.15.0", assets: ["javboss-browser-extension-v0.14.1.zip"] },
  { name: "ambiguous bundled extension assets", tag: "v2.1.4", assets: ["javboss-browser-extension-v0.15.0.zip", "javboss-browser-extension-v0.14.1.zip"] },
  { name: "missing README extension link", tag: "v2.1.4", assets: ["javboss-browser-extension-v0.15.0.zip"], missingLink: true },
  { name: "invalid release tag", tag: "../invalid", assets: [] },
]) {
  test(`${scenario.name} leaves README untouched`, (t) => {
    const dir = fs.mkdtempSync(path.join(os.tmpdir(), "javboss-readme-update-"));
    t.after(() => fs.rmSync(dir, { recursive: true, force: true }));
    const readme = path.join(dir, "README.md");
    const metadata = path.join(dir, "release.json");
    const before = appURL + "\n" + (scenario.missingLink ? "" : extURL("0.14.1"));
    fs.writeFileSync(readme, before);
    fs.writeFileSync(metadata, JSON.stringify({ assets: scenario.assets.map(name => ({ name })) }));
    const result = spawnSync(process.execPath, [script, scenario.tag, metadata, readme], { encoding: "utf8" });
    assert.notEqual(result.status, 0);
    assert.equal(fs.readFileSync(readme, "utf8"), before);
  });
}
