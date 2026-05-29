from __future__ import annotations

import shutil
import signal
import socket
import subprocess
import sys
import time
import webbrowser
from pathlib import Path


ROOT = Path(__file__).resolve().parent
BACKEND_DIR = ROOT / "backend"
FRONTEND_DIR = ROOT / "frontend"
BACKEND_PORT = 8080
FRONTEND_PORT = 3000
APP_URL = f"http://127.0.0.1:{FRONTEND_PORT}"


def find_command(*candidates: str) -> str | None:
    for candidate in candidates:
        resolved = shutil.which(candidate)
        if resolved:
            return resolved
    return None


def is_port_open(port: int) -> bool:
    with socket.socket(socket.AF_INET, socket.SOCK_STREAM) as sock:
        sock.settimeout(0.3)
        return sock.connect_ex(("127.0.0.1", port)) == 0


def wait_for_port(port: int, name: str, timeout: float = 20.0) -> None:
    deadline = time.time() + timeout
    while time.time() < deadline:
        if is_port_open(port):
            print(f"[ok] {name} is listening on {port}")
            return
        time.sleep(0.3)
    raise RuntimeError(f"{name} failed to start on port {port} within {timeout:.0f}s")


def start_backend() -> subprocess.Popen[bytes]:
    backend_exe = BACKEND_DIR / "rentadmin.exe"
    if backend_exe.exists():
        command = [str(backend_exe)]
    else:
        go_cmd = find_command("go")
        if not go_cmd:
            raise RuntimeError("Go is not installed or not in PATH")
        command = [go_cmd, "run", "."]

    print(f"[start] backend: {' '.join(command)}")
    return subprocess.Popen(command, cwd=BACKEND_DIR)


def start_frontend() -> subprocess.Popen[bytes]:
    npm_cmd = find_command("npm.cmd", "npm")
    if not npm_cmd:
        raise RuntimeError("npm is not installed or not in PATH")

    command = [npm_cmd, "run", "dev", "--", "--host", "0.0.0.0"]
    print(f"[start] frontend: {' '.join(command)}")
    return subprocess.Popen(command, cwd=FRONTEND_DIR)


def terminate_process(process: subprocess.Popen[bytes] | None, name: str) -> None:
    if process is None or process.poll() is not None:
        return

    print(f"[stop] {name}")
    process.terminate()
    try:
        process.wait(timeout=5)
    except subprocess.TimeoutExpired:
        process.kill()
        process.wait(timeout=5)


def main() -> int:
    if is_port_open(BACKEND_PORT):
        print(f"[warn] port {BACKEND_PORT} is already in use")
    if is_port_open(FRONTEND_PORT):
        print(f"[warn] port {FRONTEND_PORT} is already in use")

    backend_process: subprocess.Popen[bytes] | None = None
    frontend_process: subprocess.Popen[bytes] | None = None

    try:
        backend_process = start_backend()
        wait_for_port(BACKEND_PORT, "backend")

        frontend_process = start_frontend()
        wait_for_port(FRONTEND_PORT, "frontend")

        print(f"[ready] RentAdmin is running at {APP_URL}")
        webbrowser.open(APP_URL)

        while True:
            if backend_process.poll() is not None:
                raise RuntimeError("backend process exited unexpectedly")
            if frontend_process.poll() is not None:
                raise RuntimeError("frontend process exited unexpectedly")
            time.sleep(1)
    except KeyboardInterrupt:
        print("\n[exit] interrupted by user")
        return 0
    except Exception as exc:
        print(f"[error] {exc}")
        return 1
    finally:
        terminate_process(frontend_process, "frontend")
        terminate_process(backend_process, "backend")


if __name__ == "__main__":
    if sys.platform.startswith("win"):
        signal.signal(signal.SIGINT, signal.default_int_handler)
    raise SystemExit(main())
