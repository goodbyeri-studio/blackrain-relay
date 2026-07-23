#!/usr/bin/env node

import { randomBytes } from "node:crypto";
import {
  chmodSync,
  copyFileSync,
  existsSync,
  lstatSync,
  mkdirSync,
  readFileSync,
  rmSync,
  writeFileSync,
} from "node:fs";
import { homedir } from "node:os";
import { dirname, resolve } from "node:path";
import { spawnSync } from "node:child_process";
import { gzipSync } from "node:zlib";

const repoRoot = resolve(import.meta.dirname, "..");
const configPath = resolve(repoRoot, ".env.personal-dev");
const configExamplePath = resolve(repoRoot, ".env.personal-dev.example");
const runtimeExamplePath = resolve(
  repoRoot,
  "deploy/personal-dev/runtime.example.env",
);
const remoteComposePath = "deploy/personal-dev/docker-compose.yml";
const webComposePath = resolve(
  repoRoot,
  "deploy/personal-dev/docker-compose.web.yml",
);

function fail(message) {
  console.error(`[personal-dev] ${message}`);
  process.exit(1);
}

function readEnvFile(path) {
  if (!existsSync(path)) return {};
  const values = {};
  for (const raw of readFileSync(path, "utf8").split(/\r?\n/)) {
    const match = /^([A-Z0-9_]+)=(.*)$/.exec(raw.trim());
    if (match) values[match[1]] = match[2];
  }
  return values;
}

function expandHome(path) {
  if (path === "~") return homedir();
  if (path.startsWith("~/")) return resolve(homedir(), path.slice(2));
  return path;
}

const config = readEnvFile(configPath);
const value = (key, fallback = "") =>
  process.env[key] || config[key] || fallback;
const sshHost = value("PERSONAL_DEV_SSH_HOST", "dev-fpsmeimei");
const user = value("PERSONAL_DEV_USER", "deploy");
const sshKey = expandHome(
  value("PERSONAL_DEV_SSH_KEY", "~/.ssh/2049-personal-dev"),
);
const remoteDir = value(
  "PERSONAL_DEV_REMOTE_DIR",
  "/opt/dev/projects/meimei-api",
);
const composeProject = value(
  "PERSONAL_DEV_COMPOSE_PROJECT",
  "meimei-api-personal-dev",
);
const runtimeEnv = expandHome(
  value(
    "PERSONAL_DEV_RUNTIME_ENV_FILE",
    "~/.config/goodbyeri/personal-dev/meimei-api.env",
  ),
);
const remoteRuntimeEnv = `${remoteDir}/runtime/.env`;
const apiPort = value("PERSONAL_DEV_API_PORT", "3100");
const localApiPort = value("PERSONAL_DEV_LOCAL_API_PORT", "3310");
const webPort = value("PERSONAL_DEV_WEB_PORT", "3002");
const target = `${user}@${sshHost}`;
const tunnelSocket = `/tmp/meimei-api-personal-dev-${process.getuid?.() ?? "user"}.sock`;

function requireConfiguration() {
  if (!existsSync(configPath)) {
    fail("run `make personal-dev-init` before using personal dev");
  }
  if (!/^[A-Za-z0-9.-]+$/.test(sshHost)) {
    fail("PERSONAL_DEV_SSH_HOST must be a hostname or SSH config alias");
  }
  if (!/^[a-z_][a-z0-9_-]*$/.test(user)) {
    fail("PERSONAL_DEV_USER is invalid");
  }
  if (!/^[a-z0-9][a-z0-9_-]*$/.test(composeProject)) {
    fail("PERSONAL_DEV_COMPOSE_PROJECT is invalid");
  }
  if (
    !/^\/opt\/dev\/projects\/[A-Za-z0-9._/-]+$/.test(remoteDir) ||
    remoteDir.includes("..")
  ) {
    fail("PERSONAL_DEV_REMOTE_DIR must stay below /opt/dev/projects");
  }
  for (const port of [apiPort, localApiPort, webPort]) {
    if (!/^\d{2,5}$/.test(port) || Number(port) > 65535) {
      fail(`invalid personal dev port: ${port}`);
    }
  }
  if (!existsSync(sshKey)) fail(`SSH key not found: ${sshKey}`);
  if (!existsSync(runtimeEnv)) {
    fail(
      `runtime env not found: ${runtimeEnv}; run \`make personal-dev-init\``,
    );
  }
}

const sshArgs = [
  "-o",
  "BatchMode=yes",
  "-o",
  "StrictHostKeyChecking=yes",
  "-o",
  "Compression=yes",
  "-o",
  "ConnectTimeout=30",
  "-o",
  "ServerAliveInterval=10",
  "-o",
  "ServerAliveCountMax=3",
  "-i",
  sshKey,
];

function run(command, args, options = {}) {
  const result = spawnSync(command, args, {
    cwd: repoRoot,
    env: process.env,
    stdio: "inherit",
    ...options,
  });
  if (result.error) fail(result.error.message);
  if (result.status !== 0) process.exit(result.status ?? 1);
}

function remote(script) {
  run("ssh", [...sshArgs, target, "bash", "-s"], {
    input: script,
    stdio: ["pipe", "inherit", "inherit"],
  });
}

function capture(command, args, encoding = null) {
  const result = spawnSync(command, args, {
    cwd: repoRoot,
    encoding,
    env: process.env,
    maxBuffer: 32 * 1024 * 1024,
  });
  if (result.error) fail(result.error.message);
  if (result.status !== 0) fail(`${command} ${args[0]} failed`);
  return result.stdout;
}

function keepExistingPaths(buffer) {
  const paths = buffer.toString("utf8").split("\0").filter(Boolean);
  const existing = paths.filter((path) => {
    try {
      lstatSync(resolve(repoRoot, path));
      return true;
    } catch {
      return false;
    }
  });
  return Buffer.from(existing.length > 0 ? `${existing.join("\0")}\0` : "");
}

function init() {
  if (!existsSync(configPath)) {
    copyFileSync(configExamplePath, configPath);
    chmodSync(configPath, 0o600);
    console.log(`[personal-dev] created ${configPath}`);
  } else {
    console.log(`[personal-dev] kept existing ${configPath}`);
  }

  if (!existsSync(runtimeEnv)) {
    mkdirSync(dirname(runtimeEnv), { recursive: true, mode: 0o700 });
    const template = readFileSync(runtimeExamplePath, "utf8")
      .replace(
        "replace-with-random-development-password",
        randomBytes(24).toString("hex"),
      )
      .replace(
        "replace-with-random-development-password",
        randomBytes(24).toString("hex"),
      )
      .replace(
        "replace-with-random-development-secret",
        randomBytes(32).toString("hex"),
      )
      .replace(
        "replace-with-a-different-random-development-secret",
        randomBytes(32).toString("hex"),
      );
    writeFileSync(runtimeEnv, template, { mode: 0o600 });
    console.log(`[personal-dev] created ${runtimeEnv}`);
  } else {
    chmodSync(runtimeEnv, 0o600);
    console.log(`[personal-dev] kept existing ${runtimeEnv}`);
  }
}

function sync() {
  requireConfiguration();
  remote(`set -Eeuo pipefail
install -d -m 0750 "${remoteDir}" "${remoteDir}/source" "${remoteDir}/runtime"
`);

  const tracked = capture("git", [
    "ls-files",
    "-z",
    "--cached",
    "--others",
    "--exclude-standard",
  ]);
  const listedFiles = keepExistingPaths(tracked);
  const remoteShell = `ssh ${sshArgs.map((part) => JSON.stringify(part)).join(" ")}`;
  run(
    "rsync",
    [
      "--archive",
      "--compress",
      "--timeout=30",
      "--relative",
      "--prune-empty-dirs",
      "--from0",
      "--files-from=-",
      "--rsh",
      remoteShell,
      `${repoRoot}/`,
      `${target}:${remoteDir}/source/`,
    ],
    { input: listedFiles, stdio: ["pipe", "inherit", "inherit"] },
  );

  const manifest = keepExistingPaths(tracked)
    .toString("utf8")
    .split("\0")
    .filter(Boolean)
    .join("\n");
  run(
    "ssh",
    [...sshArgs, target, `gzip -dc > ${remoteDir}/runtime/source-manifest.txt`],
    { input: gzipSync(manifest), stdio: ["pipe", "inherit", "inherit"] },
  );
  remote(`set -Eeuo pipefail
cd "${remoteDir}/source"
{ find . -type f -print; find . -type l -print; } | sed 's#^./##' | LC_ALL=C sort > "${remoteDir}/runtime/remote-files.txt"
LC_ALL=C sort -u "${remoteDir}/runtime/source-manifest.txt" > "${remoteDir}/runtime/source-manifest.sorted.txt"
LC_ALL=C comm -23 "${remoteDir}/runtime/remote-files.txt" "${remoteDir}/runtime/source-manifest.sorted.txt" | while IFS= read -r stale; do
  case "$stale" in
    .git/*|node_modules/*|*/node_modules/*|dist/*|*/dist/*|.cache/*|*/.cache/*|data/*|logs/*) continue ;;
  esac
  if [ -w "$stale" ] || [ -w "$(dirname "$stale")" ]; then
    rm -f -- "$stale"
  fi
done
find . -depth -type d -empty -delete 2>/dev/null || true
`);
  run("rsync", [
    "--archive",
    "--timeout=30",
    "-e",
    remoteShell,
    runtimeEnv,
    `${target}:${remoteRuntimeEnv}`,
  ]);
  remote(`chmod 600 "${remoteRuntimeEnv}"`);
  console.log(`[personal-dev] synced source and runtime env to ${target}`);
}

function remoteCompose(command) {
  remote(`set -Eeuo pipefail
export PERSONAL_DEV_RUNTIME_ENV_FILE="${remoteRuntimeEnv}"
export PERSONAL_DEV_API_PORT="${apiPort}"
docker compose --project-name "${composeProject}" --env-file "${remoteRuntimeEnv}" -f "${remoteDir}/source/${remoteComposePath}" ${command}
`);
}

function up() {
  sync();
  remoteCompose("config --quiet");
  remoteCompose("up -d --build --wait");
}

function rebuild() {
  sync();
  remoteCompose("up -d --build --wait api");
}

function status() {
  requireConfiguration();
  remoteCompose("ps");
}

function logs() {
  requireConfiguration();
  remoteCompose("logs --tail=200 -f api postgres valkey");
}

function down() {
  requireConfiguration();
  webDown();
  tunnelDown();
  remoteCompose("down --remove-orphans");
}

function tunnelIsRunning() {
  if (!existsSync(tunnelSocket)) return false;
  const result = spawnSync("ssh", ["-S", tunnelSocket, "-O", "check", target], {
    cwd: repoRoot,
    stdio: "ignore",
  });
  return result.status === 0;
}

function localApiPortIsListening() {
  const result = spawnSync(
    "lsof",
    ["-nP", `-iTCP:${localApiPort}`, "-sTCP:LISTEN", "-t"],
    { cwd: repoRoot, stdio: "ignore" },
  );
  if (result.error)
    fail(`cannot inspect local tunnel port: ${result.error.message}`);
  return result.status === 0;
}

function tunnelUp() {
  requireConfiguration();
  if (tunnelIsRunning()) {
    console.log(
      `[personal-dev] tunnel already listening on 127.0.0.1:${localApiPort}`,
    );
    return;
  }
  rmSync(tunnelSocket, { force: true });
  run("ssh", [
    ...sshArgs,
    "-o",
    "ExitOnForwardFailure=yes",
    "-M",
    "-S",
    tunnelSocket,
    "-fnNT",
    "-L",
    `127.0.0.1:${localApiPort}:127.0.0.1:${apiPort}`,
    target,
  ]);
  console.log(`[personal-dev] tunnel listening on 127.0.0.1:${localApiPort}`);
}

function tunnelDown() {
  if (!tunnelIsRunning()) {
    rmSync(tunnelSocket, { force: true });
  } else {
    run("ssh", ["-S", tunnelSocket, "-O", "exit", target]);
    rmSync(tunnelSocket, { force: true });
  }

  for (let attempt = 0; attempt < 20 && localApiPortIsListening(); attempt++) {
    Atomics.wait(new Int32Array(new SharedArrayBuffer(4)), 0, 0, 100);
  }
  if (localApiPortIsListening()) {
    fail(`local API port 127.0.0.1:${localApiPort} is still listening`);
  }
  console.log(`[personal-dev] tunnel stopped on 127.0.0.1:${localApiPort}`);
}

function webCompose(args) {
  requireConfiguration();
  run("docker", [
    "compose",
    "--env-file",
    configPath,
    "-f",
    webComposePath,
    ...args,
  ]);
}

function webUp() {
  tunnelUp();
  webCompose(["up", "-d", "--wait", "web-dev"]);
  console.log(`[personal-dev] frontend ready at http://127.0.0.1:${webPort}`);
}

function webStatus() {
  webCompose(["ps"]);
}

function webLogs() {
  webCompose(["logs", "--tail=200", "-f", "web-dev"]);
}

function webDown() {
  if (!existsSync(configPath)) return;
  webCompose(["down", "--remove-orphans"]);
}

function doctor() {
  requireConfiguration();
  status();
  tunnelUp();
  run("curl", [
    "--fail",
    "--silent",
    "--show-error",
    `http://127.0.0.1:${localApiPort}/healthz/ready`,
  ]);
  console.log(`\n[personal-dev] backend healthy through SSH tunnel`);
}

function start() {
  up();
  webUp();
  doctor();
}

const commands = {
  init,
  start,
  sync,
  rebuild,
  status,
  logs,
  doctor,
  down,
  "tunnel-up": tunnelUp,
  "tunnel-down": tunnelDown,
  "web-up": webUp,
  "web-status": webStatus,
  "web-logs": webLogs,
  "web-down": webDown,
};
const requested = process.argv[2] || "doctor";
if (!commands[requested]) {
  fail(`unknown command ${requested}; use ${Object.keys(commands).join(", ")}`);
}
commands[requested]();
