#!/usr/bin/env node
"use strict";

// Downloads the release binary for this platform from the GitHub release of
// the package version, checks its SHA-256 against checksums.txt and unpacks it
// into ./bin.

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
const MAX_REDIRECTS = 10;
const TIMEOUT_MS = 30_000;

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

function download(url, redirects = 0) {
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
        download(next, redirects + 1).then(resolve, reject);
        return;
      }
      if (res.statusCode !== 200) {
        res.resume();
        reject(new Error(`download of ${url} failed: HTTP ${res.statusCode}`));
        return;
      }
      const chunks = [];
      res.on("data", (c) => chunks.push(c));
      res.on("end", () => resolve(Buffer.concat(chunks)));
      res.on("error", reject);
    });
    req.on("timeout", () => req.destroy(new Error(`timed out after ${TIMEOUT_MS} ms: ${url}`)));
    req.on("error", reject);
  });
}

function verify(buffer, asset, checksums) {
  const line = checksums
    .split("\n")
    .map((l) => l.trim().split(/\s+/))
    .find((parts) => parts[1] === asset);
  if (!line) {
    throw new Error(`checksums.txt has no entry for ${asset}`);
  }
  const actual = crypto.createHash("sha256").update(buffer).digest("hex");
  if (actual !== line[0]) {
    throw new Error(`checksum mismatch for ${asset}: expected ${line[0]}, got ${actual}`);
  }
}

function extract(archive) {
  if (archive.endsWith(".zip")) {
    const q = (s) => `'${s.replace(/'/g, "''")}'`;
    execFileSync(
      "powershell",
      ["-NoProfile", "-NonInteractive", "-Command", `Expand-Archive -Force -LiteralPath ${q(archive)} -DestinationPath ${q(BIN_DIR)}`],
      { stdio: "inherit" }
    );
  } else {
    execFileSync("tar", ["-xzf", archive, "-C", BIN_DIR, BIN_NAME], { stdio: "inherit" });
  }
  if (process.platform !== "win32") {
    fs.chmodSync(BIN_PATH, 0o755);
  }
}

async function main() {
  if (fs.existsSync(BIN_PATH)) {
    return;
  }
  const asset = assetName();
  const base = `https://github.com/${REPO}/releases/download/v${VERSION}`;
  console.log(`Downloading ${NAME} v${VERSION} (${asset})...`);
  const [archive, checksums] = await Promise.all([download(`${base}/${asset}`), download(`${base}/checksums.txt`)]);
  verify(archive, asset, checksums.toString("utf8"));
  fs.mkdirSync(BIN_DIR, { recursive: true });
  const archivePath = path.join(BIN_DIR, asset);
  try {
    fs.writeFileSync(archivePath, archive);
    extract(archivePath);
  } finally {
    fs.rmSync(archivePath, { force: true });
  }
  console.log(`Installed ${NAME} to ${BIN_PATH}`);
}

if (require.main === module) {
  main().catch((err) => {
    console.error(`Failed to install ${NAME}: ${err.message}`);
    process.exit(1);
  });
}

module.exports = { assetName, download, verify };
