"use strict";

// Tests for the npm wrapper's download checks: node --test

const assert = require("node:assert");
const crypto = require("node:crypto");
const { EventEmitter } = require("node:events");
const https = require("node:https");
const { test, mock, afterEach } = require("node:test");

const { assetName, download, verify } = require("./postinstall.js");

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
  verify(archive, "a.tar.gz", `${sha(archive)}  a.tar.gz\n`);
});

test("a tampered archive fails", () => {
  const archive = Buffer.from("real archive");
  const sums = `${sha(archive)}  a.tar.gz\n`;
  assert.throws(() => verify(Buffer.from("tampered"), "a.tar.gz", sums), /checksum mismatch for a\.tar\.gz/);
});

test("a wrong sum fails", () => {
  assert.throws(() => verify(Buffer.from("x"), "a.tar.gz", `${"0".repeat(64)}  a.tar.gz\n`), /checksum mismatch/);
});

test("an archive missing from checksums.txt fails", () => {
  const archive = Buffer.from("x");
  assert.throws(() => verify(archive, "a.tar.gz", `${sha(archive)}  b.tar.gz\n`), /no entry for a\.tar\.gz/);
});

test("a checksums.txt that answers 404 fails", async () => {
  fakeGet({ "https://example.test/checksums.txt": { status: 404 } });
  await assert.rejects(download("https://example.test/checksums.txt"), /HTTP 404/);
});

test("a redirect to plain HTTP is refused", async () => {
  fakeGet({ "https://example.test/a.tar.gz": { status: 302, location: "http://example.test/a.tar.gz" } });
  await assert.rejects(download("https://example.test/a.tar.gz"), /refusing a non-HTTPS URL/);
});

test("an HTTPS redirect is followed", async () => {
  fakeGet({
    "https://example.test/a.tar.gz": { status: 302, location: "https://cdn.example.test/a.tar.gz" },
    "https://cdn.example.test/a.tar.gz": { status: 200, body: "bytes" },
  });
  assert.strictEqual((await download("https://example.test/a.tar.gz")).toString(), "bytes");
});
