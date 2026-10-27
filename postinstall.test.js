"use strict";

// Tests for the npm wrapper's download checks: node --test

const assert = require("node:assert");
const crypto = require("node:crypto");
const { EventEmitter } = require("node:events");
const https = require("node:https");
const { test, mock, afterEach } = require("node:test");

const fs = require("node:fs");
const os = require("node:os");
const path = require("node:path");
const { execFileSync } = require("node:child_process");
const { assetName, download, expectedSum, verify, extract, install, BIN_PATH } = require("./postinstall.js");

const tmpFile = () => path.join(fs.mkdtempSync(path.join(os.tmpdir(), "bam-")), "a.tar.gz");

const sha = (b) => crypto.createHash("sha256").update(b).digest("hex");

afterEach(() => mock.restoreAll());

// fakeGet answers https.get from a table of URL -> {status, location, body}.
function fakeGet(table) {
  mock.method(https, "get", (url, _opts, cb) => {
    const req = new EventEmitter();
    process.nextTick(() => {
      const r = table[url];
      if (!r) {
        req.emit("error", new Error(`unexpected request ${url}`));
        return;
      }
      const res = new EventEmitter();
      res.statusCode = r.status;
      res.headers = r.location ? { location: r.location } : {};
      res.resume = () => {};
      res.destroy = () => {};
      cb(res);
      if (r.body) res.emit("data", Buffer.from(r.body));
      res.emit("end");
    });
    return req;
  });
}

test("asset name carries version, platform and arch", () => {
  assert.match(assetName(), /^biblical-atlas-mcp_\d+\.\d+\.\d+_(linux|darwin|windows)_(amd64|arm64)\.(tar\.gz|zip)$/);
});

test("a matching archive passes", () => {
  const archive = Buffer.from("real archive");
  verify(sha(archive), "a.tar.gz", `${sha(archive)}  a.tar.gz\n`);
});

test("a tampered archive fails", () => {
  const sums = `${sha(Buffer.from("real archive"))}  a.tar.gz\n`;
  assert.throws(() => verify(sha(Buffer.from("tampered")), "a.tar.gz", sums), /checksum mismatch for a\.tar\.gz/);
});

test("a wrong sum fails", () => {
  assert.throws(() => verify(sha(Buffer.from("x")), "a.tar.gz", `${"0".repeat(64)}  a.tar.gz\n`), /checksum mismatch/);
});

test("an archive missing from checksums.txt fails", () => {
  const archive = Buffer.from("x");
  assert.throws(() => verify(sha(archive), "a.tar.gz", `${sha(archive)}  b.tar.gz\n`), /no entry for a\.tar\.gz/);
});

test("a malformed sum is no entry", () => {
  assert.throws(() => expectedSum("a.tar.gz", "nothex  a.tar.gz\n"), /no entry for a\.tar\.gz/);
});

test("a download that answers 404 fails", async () => {
  fakeGet({ "https://example.test/a.tar.gz": { status: 404 } });
  await assert.rejects(download("https://example.test/a.tar.gz", tmpFile()), /HTTP 404/);
});

test("a redirect to plain HTTP is refused", async () => {
  fakeGet({ "https://example.test/a.tar.gz": { status: 302, location: "http://example.test/a.tar.gz" } });
  await assert.rejects(download("https://example.test/a.tar.gz", tmpFile()), /refusing a non-HTTPS URL/);
});

test("an HTTPS redirect is followed and the file hashed as written", async () => {
  fakeGet({
    "https://example.test/a.tar.gz": { status: 302, location: "https://cdn.example.test/a.tar.gz" },
    "https://cdn.example.test/a.tar.gz": { status: 200, body: "bytes" },
  });
  const dest = tmpFile();
  const sum = await download("https://example.test/a.tar.gz", dest);
  assert.strictEqual(sum, sha(Buffer.from("bytes")));
  assert.strictEqual(fs.readFileSync(dest, "utf8"), "bytes");
});

test("a download over the size cap stops and leaves no file", async () => {
  fakeGet({ "https://example.test/a.tar.gz": { status: 200, body: "x".repeat(100) } });
  const dest = tmpFile();
  await assert.rejects(download("https://example.test/a.tar.gz", dest, { maxBytes: 10 }), /larger than 10 bytes/);
  assert.strictEqual(fs.existsSync(dest), false);
});

test("install refuses to run without the bundled checksums.txt", async () => {
  if (fs.existsSync(path.join(__dirname, "checksums.txt")) || fs.existsSync(BIN_PATH)) {
    return; // a packed copy: nothing to prove here
  }
  fakeGet({});
  await assert.rejects(install(), /checksums\.txt is missing/);
});

test("a truncated archive leaves no binary behind", (t) => {
  if (process.platform === "win32" || fs.existsSync(BIN_PATH)) {
    t.skip("needs tar and an empty bin directory");
    return;
  }
  const binDir = path.dirname(BIN_PATH);
  const hadBinDir = fs.existsSync(binDir);
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), "ba-extract-"));
  t.after(() => {
    fs.rmSync(dir, { recursive: true, force: true });
    if (!hadBinDir) fs.rmSync(binDir, { recursive: true, force: true });
  });
  // Random bytes do not compress, so cutting the archive in half cuts the
  // binary in half too, and tar writes part of it before it fails.
  fs.writeFileSync(path.join(dir, path.basename(BIN_PATH)), crypto.randomBytes(1 << 20));
  const archive = path.join(dir, "a.tar.gz");
  execFileSync("tar", ["-czf", archive, "-C", dir, path.basename(BIN_PATH)]);
  const whole = fs.readFileSync(archive);
  fs.writeFileSync(archive, whole.subarray(0, whole.length >> 1));
  assert.throws(() => extract(archive));
  assert.strictEqual(fs.existsSync(BIN_PATH), false);
});
