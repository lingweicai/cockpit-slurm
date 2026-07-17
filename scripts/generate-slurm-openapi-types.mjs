#!/usr/bin/env node

import { mkdirSync, readFileSync, writeFileSync } from "node:fs";
import { dirname, resolve } from "node:path";
import { fileURLToPath } from "node:url";
import { spawnSync } from "node:child_process";

const __filename = fileURLToPath(import.meta.url);
const __dirname = dirname(__filename);
const repoRoot = resolve(__dirname, "..");
const configPath = resolve(__dirname, "slurm-openapi-types.config.json");

const isDryRun = process.argv.includes("--dry-run");

const config = JSON.parse(readFileSync(configPath, "utf8"));
const input = resolve(repoRoot, config.input);
const output = resolve(repoRoot, config.output);
const outputDir = dirname(output);
const options = Array.isArray(config.options) ? config.options : [];

mkdirSync(outputDir, { recursive: true });

const args = [input, "-o", output, ...options];

if (isDryRun) {
  console.log("openapi-typescript", args.join(" "));
  process.exit(0);
}

const result = spawnSync("openapi-typescript", args, {
  cwd: repoRoot,
  stdio: "inherit",
  shell: true,
});

if (result.status !== 0) {
  process.exit(result.status ?? 1);
}

const generated = readFileSync(output, "utf8");
if (!generated.startsWith("/* eslint-disable */")) {
  writeFileSync(output, `/* eslint-disable */\n${generated}`);
}
