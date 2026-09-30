#!/usr/bin/env node
"use strict";

// Starts the downloaded binary in stdio mode, forwards signals and exits
// with the child's status.

const { spawn } = require("child_process");
const fs = require("fs");
const os = require("os");
const path = require("path");

const NAME = "biblical-atlas-mcp";
const BIN_PATH = path.join(__dirname, "bin", process.platform === "win32" ? `${NAME}.exe` : NAME);

if (!fs.existsSync(BIN_PATH)) {
  console.error(`${NAME} binary not found at ${BIN_PATH}. Reinstall the package or run: node ${path.join(__dirname, "postinstall.js")}`);
  process.exit(1);
}

const child = spawn(BIN_PATH, process.argv.slice(2), {
  stdio: "inherit",
  env: Object.assign({}, process.env, { TRANSPORT: "stdio" }),
});

for (const sig of ["SIGINT", "SIGTERM", "SIGHUP"]) {
  process.on(sig, () => {
    if (!child.killed) {
      child.kill(sig);
    }
  });
}

child.on("error", (err) => {
  console.error(`${NAME}: ${err.message}`);
  process.exit(1);
});

child.on("exit", (code, signal) => {
  if (signal) {
    process.exit(128 + (os.constants.signals[signal] || 0));
  }
  process.exit(code === null ? 1 : code);
});
