#!/usr/bin/env node
"use strict";

// Starts the downloaded binary in stdio mode, forwards signals and exits
// with the child's status. When the binary is missing, because npm 12, pnpm
// or Bun skipped the postinstall script, it downloads it first (on stderr;
// stdout belongs to the MCP client).

const { spawn } = require("child_process");
const os = require("os");
const { install, BIN_PATH } = require("./postinstall.js");

const NAME = "biblical-atlas-mcp";

async function start() {
  try {
    await install();
  } catch (err) {
    console.error(`${NAME}: the binary is not installed and the download failed: ${err.message}`);
    process.exit(1);
  }
  const child = spawn(BIN_PATH, process.argv.slice(2), {
    stdio: "inherit",
    env: Object.assign({}, process.env, { TRANSPORT: "stdio" }),
  });
  forward(child);
}

function forward(child) {
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
}

start();
