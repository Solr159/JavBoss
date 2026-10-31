#!/usr/bin/env python3
"""Run a real, isolated Linux x86_64 update before publishing anything to GitHub.

python3 scripts/test-self-update.py            # automatic API-driven verification
python3 scripts/test-self-update.py --manual   # click Update in the real frontend

Requires the normal Go/frontend build dependencies and Python 3. No pip packages.
All artifacts and logs remain under temp/ for inspection. Ctrl+C stops test processes.
"""

import argparse
import hashlib
import http.cookiejar
import http.server
import json
import os
from pathlib import Path
import platform
import shutil
import signal
import socket
import subprocess
import tempfile
import threading
import time
import urllib.error
import urllib.request
import zipfile


ROOT = Path(__file__).resolve().parents[1]
OLD, NEW = "v0.0.1", "v0.0.2"
PREFIX = "/Solr159/JavBoss/releases"
NOTES = "Local self-update test: replacement, restart and cleanup."
ARCHIVE_NAME = f"javboss-{NEW}-linux-x86_64.zip"


def say(message):
    print(message, flush=True)


def require(condition, message):
    if not condition:
        raise RuntimeError(message)


def wait_for(description, check, timeout=60):
    deadline = time.monotonic() + timeout
    last_error = None
    while time.monotonic() < deadline:
        try:
            value = check()
            if value:
                return value
        except (OSError, urllib.error.URLError, json.JSONDecodeError) as error:
            last_error = error
        time.sleep(0.2)
    raise RuntimeError(f"等待{description}超时；最后错误：{last_error}")


def test_processes(workspace):
    # Match executable paths, never names/ports, so other installations are untouched.
    result = []
    for entry in Path("/proc").iterdir():
        if not entry.name.isdigit():
            continue
        try:
            executable = os.readlink(entry / "exe")
            if executable.startswith(str(workspace) + "/"):
                result.append(int(entry.name))
        except OSError:
            pass
    return result


def stop_test_processes(workspace):
    for sig in (signal.SIGTERM, signal.SIGKILL):
        for pid in test_processes(workspace):
            try:
                os.kill(pid, sig)
            except ProcessLookupError:
                pass
        deadline = time.monotonic() + 5
        while test_processes(workspace) and time.monotonic() < deadline:
            time.sleep(0.1)
    require(not test_processes(workspace), "测试进程未能退出")


def build(workspace, reuse_frontend):
    if not reuse_frontend:
        say("构建当前前端…")
        subprocess.run(["npm", "run", "build"], cwd=ROOT / "web", check=True)
    require((ROOT / "web/dist/index.html").is_file(), "缺少 web/dist，请先构建前端")
    env = {**os.environ, "GOCACHE": str(ROOT / ".gocache"), "CGO_ENABLED": "1",
           "GOOS": "linux", "GOARCH": "amd64"}
    for version, directory in ((OLD, workspace / "installed"), (NEW, workspace / "payload")):
        directory.mkdir()
        shutil.copytree(ROOT / "web/dist", directory / "web/dist")
        (directory / "web/dist/update-test-version.txt").write_text(version)
        (directory / "config.toml").write_text('server_url = ""\nport = 8655\n')
        say(f"构建当前后端，测试版本 {version}…")
        subprocess.run([
            "go", "build", "-tags", "updatetest", "-ldflags",
            f"-X main.buildMode=release -X javboss/internal/common.Version={version}",
            "-o", str(directory / "javboss"), "./cmd/server",
        ], cwd=ROOT, env=env, check=True)
    archive = workspace / ARCHIVE_NAME
    with zipfile.ZipFile(archive, "w", zipfile.ZIP_DEFLATED) as package:
        for file in sorted((workspace / "payload").rglob("*")):
            if file.is_file():
                package.write(file, "JavBoss/" + str(file.relative_to(workspace / "payload")))
    return archive


def release_server(archive, workspace):
    requests = []

    class Handler(http.server.BaseHTTPRequestHandler):
        def log_message(self, fmt, *args):
            with (workspace / "release-server.log").open("a") as log:
                log.write((fmt % args) + "\n")

        def do_HEAD(self):
            requests.append(("HEAD", self.path))
            if self.path == PREFIX + "/latest":
                self.send_response(302)
                self.send_header("Location", f"https://github.com{PREFIX}/tag/{NEW}")
                self.end_headers()
            else:
                self.send_error(404)

        def do_GET(self):
            requests.append(("GET", self.path))
            if self.path == f"{PREFIX}/tag/{NEW}":
                body = (f'<div data-test-selector="body-content" '
                        f'class="markdown-body"><p>{NOTES}</p></div>').encode()
                self.send_response(200)
                self.send_header("Content-Type", "text/html; charset=utf-8")
                self.send_header("Content-Length", str(len(body)))
                self.end_headers()
                self.wfile.write(body)
            elif self.path == f"{PREFIX}/download/{NEW}/{ARCHIVE_NAME}":
                self.send_response(200)
                self.send_header("Content-Type", "application/zip")
                self.send_header("Content-Length", str(archive.stat().st_size))
                self.end_headers()
                with archive.open("rb") as source:
                    shutil.copyfileobj(source, self.wfile)
            else:
                self.send_error(404)

    server = http.server.ThreadingHTTPServer(("127.0.0.1", 0), Handler)
    threading.Thread(target=server.serve_forever, daemon=True).start()
    return server, requests


def verify_update(args, workspace, server, requests):
    installed = workspace / "installed"
    with socket.socket() as sock:
        sock.bind(("127.0.0.1", 0))
        port = sock.getsockname()[1]
    base_url = f"http://127.0.0.1:{port}"
    env = dict(os.environ)
    for key in ("JAVBOSS_CONTAINER", "JAVBOSS_UPDATE_READY", "JAVBOSS_BACKGROUND_CHILD"):
        env.pop(key, None)
    env["JAVBOSS_UPDATE_TEST_URL"] = f"http://127.0.0.1:{server.server_port}"
    opener = urllib.request.build_opener(urllib.request.ProxyHandler({}),
                                        urllib.request.HTTPCookieProcessor(http.cookiejar.CookieJar()))

    def api(path, method="GET", body=None):
        request = urllib.request.Request(base_url + path, method=method,
                                         data=json.dumps(body).encode() if body is not None else None,
                                         headers={"Content-Type": "application/json", "Origin": base_url})
        with opener.open(request, timeout=3) as response:
            return json.load(response)

    with (workspace / "initial-process.log").open("w") as log:
        old = subprocess.Popen([str(installed / "javboss"), "-port", str(port)],
                               cwd=installed, env=env, stdin=subprocess.DEVNULL,
                               stdout=log, stderr=log)
        try:
            wait_for("初始程序启动", lambda: api("/healthz"))
            api("/auth/login", "POST", {"password": "admin"})
            api("/config", "PATCH", {"video_page_size": 37})
            config_before = (installed / "config.toml").read_bytes()
            (installed / "data/update-test-user-file.txt").write_text("preserve me")
            status = wait_for("检查到本地新版本", lambda: (
                value if (value := api("/system/update")).get("can_install") else None))
            require(status["current_version"] == OLD and status["latest_version"] == NEW,
                    f"版本检查不符合预期：{status}")
            require(NOTES in status.get("notes", ""), "未获取到本地更新说明")
            if args.manual:
                say(f"\n打开 {base_url}，默认密码 admin。进入设置 → 更新，点击更新并确认。")
                say("脚本将验证升级结果；完成后继续保留页面，按 Ctrl+C 停止。")
            else:
                say("通过真实 API 触发更新…")
                api("/system/update/install", "POST")

            def updated():
                status = api("/system/update")
                if status.get("phase") == "error":
                    raise RuntimeError(f"更新失败：{status}")
                result = status.get("last_result") or {}
                if result.get("error"):
                    raise RuntimeError(f"助手报告更新失败：{result}")
                return status if status.get("current_version") == NEW else None

            wait_for("新版本启动", updated, timeout=1800 if args.manual else 180)
            require(old.wait(timeout=15) == 0, "旧程序未正常退出")
            wait_for("更新目录清理", lambda: not (installed / ".updates").exists())
            result = json.loads((installed / "data/update-result.json").read_text())
            require(result["version"] == NEW and result["finished_at"] > 0 and not result.get("error"),
                    f"更新结果异常：{result}")
            require(api("/config")["video_page_size"] == "37", "数据库设置未保留")
            require((installed / "config.toml").read_bytes() == config_before, "配置文件被覆盖")
            require((installed / "data/update-test-user-file.txt").read_text() == "preserve me",
                    "用户文件被覆盖")
            require((installed / ".version").read_text().strip() == NEW, "版本文件错误")
            require((installed / "data/update-operation.lock").is_file(), "操作锁文件被删除")
            for relative in ("javboss", "web/dist/index.html", "web/dist/update-test-version.txt"):
                actual = hashlib.sha256((installed / relative).read_bytes()).digest()
                expected = hashlib.sha256((workspace / "payload" / relative).read_bytes()).digest()
                require(actual == expected, f"文件没有更新：{relative}")
            for request in (("HEAD", PREFIX + "/latest"), ("GET", f"{PREFIX}/tag/{NEW}"),
                            ("GET", f"{PREFIX}/download/{NEW}/{ARCHIVE_NAME}")):
                require(request in requests, f"未执行发布请求：{request}")
            say("通过：本地版本检查、更新说明、ZIP 下载、文件替换、真实重启、数据保留、结果记录和 .updates 清理。")
            if args.manual:
                say(f"新版本仍运行于 {base_url}；按 Ctrl+C 结束测试。")
                while True:
                    time.sleep(1)
        finally:
            stop_test_processes(workspace)
            old.wait(timeout=10)


def main():
    parser = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    parser.add_argument("--manual", action="store_true", help="在真实前端手动触发更新")
    parser.add_argument("--reuse-frontend", action="store_true", help="复用现有 web/dist，跳过前端构建")
    args = parser.parse_args()
    require(platform.system() == "Linux" and platform.machine() == "x86_64",
            "此脚本目前支持 Linux x86_64；其他平台可运行 Go 更新模块测试")
    (ROOT / "temp").mkdir(exist_ok=True)
    workspace = Path(tempfile.mkdtemp(prefix="self-update-", dir=ROOT / "temp"))
    say(f"测试目录：{workspace}\n仅使用本地测试更新源，无需发布 GitHub Release。")
    try:
        archive = build(workspace, args.reuse_frontend)
        server, requests = release_server(archive, workspace)
        try:
            verify_update(args, workspace, server, requests)
        finally:
            server.shutdown()
            server.server_close()
    finally:
        say(f"日志和测试数据保留在：{workspace}")


if __name__ == "__main__":
    try:
        main()
    except KeyboardInterrupt:
        say("测试已停止。")
        raise SystemExit(130)
    except (OSError, RuntimeError, subprocess.SubprocessError) as error:
        say(f"测试失败：{error}")
        raise SystemExit(1)
