#!/usr/bin/env node
"use strict";

// Downloads the release binary for this platform from the GitHub release of
// the package version, checks its SHA-256 against the checksums.txt bundled
// in this npm package and unpacks it into ./bin.
//
// The checksums come with the package, not from the release: release.yml
// copies GoReleaser's checksums.txt into the package before npm publish, so
// npm's provenance covers it, and someone who can replace release assets
// cannot replace the sums that check them.
//
// run.js calls install() when ./bin is empty, because npm 12, pnpm and Bun
// skip install scripts by default. Everything here logs to stderr: on that
// path stdout belongs to the MCP client.

const { execFileSync } = require("child_process");
const crypto = require("crypto");
const fs = require("fs");
const https = require("https");
const path = require("path");

const VERSION = require("./package.json").version;
const REPO = "GeiserX/biblical-atlas-mcp";
const NAME = "biblical-atlas-mcp";
const BIN_NAME = process.platform === "win32" ? `${NAME}.exe` : NAME;
const BIN_DIR = path.join(__dirname, "bin");
const BIN_PATH = path.join(BIN_DIR, BIN_NAME);
const CHECKSUMS = path.join(__dirname, "checksums.txt");
const MAX_REDIRECTS = 10;
const TIMEOUT_MS = 30_000;
// The archives are about 5 MB; anything far larger is not ours.
const MAX_BYTES = 64 * 1024 * 1024;

const PLATFORMS = { darwin: "darwin", linux: "linux", win32: "windows" };
const ARCHES = { x64: "amd64", arm64: "arm64" };

function assetName() {
  const platform = PLATFORMS[process.platform];
  const arch = ARCHES[process.arch];
  if (!platform || !arch) {
    throw new Error(`unsupported platform ${process.platform}-${process.arch}`);
  }
  const ext = platform === "windows" ? "zip" : "tar.gz";
  return `${NAME}_${VERSION}_${platform}_${arch}.${ext}`;
}

// download streams url to dest, following HTTPS redirects only, and
// resolves with the SHA-256 of what it wrote. A body over maxBytes stops the
// download and removes dest.
function download(url, dest, { maxBytes = MAX_BYTES, redirects = 0 } = {}) {
  return new Promise((resolve, reject) => {
    if (!url.startsWith("https://")) {
      reject(new Error(`refusing a non-HTTPS URL: ${url}`));
      return;
    }
    const req = https.get(url, { timeout: TIMEOUT_MS }, (res) => {
      if (res.statusCode >= 300 && res.statusCode < 400 && res.headers.location) {
        res.resume();
        if (redirects >= MAX_REDIRECTS) {
          reject(new Error(`more than ${MAX_REDIRECTS} redirects`));
          return;
        }
        const next = new URL(res.headers.location, url).toString();
        download(next, dest, { maxBytes, redirects: redirects + 1 }).then(resolve, reject);
        return;
      }
      if (res.statusCode !== 200) {
        res.resume();
        reject(new Error(`download of ${url} failed: HTTP ${res.statusCode}`));
        return;
      }
      const hash = crypto.createHash("sha256");
      const out = fs.createWriteStream(dest, { mode: 0o600 });
      let size = 0;
      let failed = false;
      const fail = (err) => {
        if (failed) return;
        failed = true;
        res.destroy();
        // The file may open after this point; remove it once it is closed.
        out.once("close", () => {
          fs.rmSync(dest, { force: true });
          reject(err);
        });
        out.destroy();
      };
      res.on("data", (chunk) => {
        size += chunk.length;
        if (size > maxBytes) {
          fail(new Error(`download of ${url} is larger than ${maxBytes} bytes`));
          return;
        }
        hash.update(chunk);
        out.write(chunk);
      });
      res.on("end", () => {
        if (failed) return;
        out.end(() => resolve(hash.digest("hex")));
      });
      res.on("error", fail);
      out.on("error", fail);
    });
    req.on("timeout", () => req.destroy(new Error(`timed out after ${TIMEOUT_MS} ms: ${url}`)));
    req.on("error", reject);
  });
}

// expectedSum reads the sum for asset from checksums.txt text.
function expectedSum(asset, checksums) {
  const line = checksums
    .split("\n")
    .map((l) => l.trim().split(/\s+/))
    .find((parts) => parts[1] === asset);
  if (!line || !/^[0-9a-f]{64}$/.test(line[0])) {
    throw new Error(`checksums.txt has no entry for ${asset}`);
  }
  return line[0];
}

function verify(actual, asset, checksums) {
  const want = expectedSum(asset, checksums);
  if (actual !== want) {
    throw new Error(`checksum mismatch for ${asset}: expected ${want}, got ${actual}`);
  }
}

function bundledChecksums() {
  if (!fs.existsSync(CHECKSUMS)) {
    throw new Error(
      `${CHECKSUMS} is missing: this copy of ${NAME} was not installed from npm. Install it from npm, or download a release binary and check it by hand.`
    );
  }
  return fs.readFileSync(CHECKSUMS, "utf8");
}

function extract(archive) {
  if (archive.endsWith(".zip")) {
    const q = (s) => `'${s.replace(/'/g, "''")}'`;
    execFileSync(
      "powershell",
      ["-NoProfile", "-NonInteractive", "-Command", `Expand-Archive -Force -LiteralPath ${q(archive)} -DestinationPath ${q(BIN_DIR)}`],
      { stdio: ["ignore", "ignore", "inherit"] }
    );
  } else {
    execFileSync("tar", ["-xzf", archive, "-C", BIN_DIR, BIN_NAME], { stdio: ["ignore", "ignore", "inherit"] });
  }
  if (process.platform !== "win32") {
    fs.chmodSync(BIN_PATH, 0o755);
  }
}

async function install() {
  if (fs.existsSync(BIN_PATH)) {
    return;
  }
  const asset = assetName();
  const checksums = bundledChecksums();
  expectedSum(asset, checksums);
  const url = `https://github.com/${REPO}/releases/download/v${VERSION}/${asset}`;
  console.error(`Downloading ${NAME} v${VERSION} (${asset})...`);
  fs.mkdirSync(BIN_DIR, { recursive: true });
  const archivePath = path.join(BIN_DIR, `${asset}.part`);
  try {
    verify(await download(url, archivePath), asset, checksums);
    const final = path.join(BIN_DIR, asset);
    fs.renameSync(archivePath, final);
    try {
      extract(final);
    } finally {
      fs.rmSync(final, { force: true });
    }
  } finally {
    fs.rmSync(archivePath, { force: true });
  }
  console.error(`Installed ${NAME} to ${BIN_PATH}`);
}

if (require.main === module) {
  install().catch((err) => {
    console.error(`Failed to install ${NAME}: ${err.message}`);
    process.exit(1);
  });
}

module.exports = { assetName, download, expectedSum, verify, install, BIN_PATH };
