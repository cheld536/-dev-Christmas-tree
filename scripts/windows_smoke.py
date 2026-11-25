import argparse
import ctypes
from ctypes import wintypes
import json
import os
from pathlib import Path
import subprocess
import tempfile
import time
import winreg


USER32 = ctypes.WinDLL("user32", use_last_error=True)
ENUM = ctypes.WINFUNCTYPE(wintypes.BOOL, wintypes.HWND, wintypes.LPARAM)
USER32.EnumWindows.argtypes = [ENUM, wintypes.LPARAM]
USER32.EnumChildWindows.argtypes = [wintypes.HWND, ENUM, wintypes.LPARAM]
USER32.GetWindowThreadProcessId.argtypes = [wintypes.HWND, ctypes.POINTER(wintypes.DWORD)]
USER32.GetClassNameW.argtypes = [wintypes.HWND, wintypes.LPWSTR, ctypes.c_int]
USER32.IsWindowVisible.argtypes = [wintypes.HWND]
USER32.GetDlgItem.argtypes = [wintypes.HWND, ctypes.c_int]
USER32.GetDlgItem.restype = wintypes.HWND
USER32.PostMessageW.argtypes = [wintypes.HWND, wintypes.UINT, wintypes.WPARAM, wintypes.LPARAM]
USER32.SendMessageW.argtypes = [wintypes.HWND, wintypes.UINT, wintypes.WPARAM, wintypes.LPARAM]
USER32.SendMessageW.restype = wintypes.LPARAM
USER32.SendMessageTimeoutW.argtypes = [
    wintypes.HWND, wintypes.UINT, wintypes.WPARAM, wintypes.LPARAM,
    wintypes.UINT, wintypes.UINT, ctypes.POINTER(wintypes.WPARAM),
]
USER32.IsWindow.argtypes = [wintypes.HWND]
SHELL = ctypes.WinDLL("shell32", use_last_error=True)
SHELL.ExtractIconExW.argtypes = [wintypes.LPCWSTR, ctypes.c_int, ctypes.c_void_p, ctypes.c_void_p, wintypes.UINT]
SHELL.ExtractIconExW.restype = wintypes.UINT

UNINSTALL = r"Software\Microsoft\Windows\CurrentVersion\Uninstall\DeveloperChristmasTree"
DESKTOP = r"Control Panel\Desktop"


def wait(check, description, seconds=30):
    deadline = time.monotonic() + seconds
    while time.monotonic() < deadline:
        if result := check():
            return result
        time.sleep(0.12)
    raise AssertionError(f"시간 초과 — {description}")


def visible_windows(process, class_name):
    result = []

    @ENUM
    def receive(hwnd, _):
        owner = wintypes.DWORD()
        USER32.GetWindowThreadProcessId(hwnd, ctypes.byref(owner))
        if owner.value == process.pid and USER32.IsWindowVisible(hwnd):
            klass = ctypes.create_unicode_buffer(70)
            USER32.GetClassNameW(hwnd, klass, len(klass))
            if klass.value == class_name:
                result.append(hwnd)
        return True

    USER32.EnumWindows(receive, 0)
    return result


def window(process, class_name):
    return wait(lambda: next(iter(visible_windows(process, class_name)), None), class_name)


def text(hwnd):
    buffer = ctypes.create_unicode_buffer(2048)
    length = wintypes.WPARAM()
    succeeded = USER32.SendMessageTimeoutW(
        hwnd, 0xD, len(buffer), ctypes.addressof(buffer), 2, 1000, ctypes.byref(length)
    )
    return buffer.value if succeeded else ""


def post(hwnd, message, value=0):
    if not USER32.PostMessageW(hwnd, message, value, 0):
        raise ctypes.WinError(ctypes.get_last_error())


def command(hwnd, control, notification=0):
    post(hwnd, 0x111, control | (notification << 16))


def registry(path, name):
    try:
        with winreg.OpenKey(winreg.HKEY_CURRENT_USER, path) as key:
            return winreg.QueryValueEx(key, name)
    except FileNotFoundError:
        return None


def desktop_values():
    return {name: registry(DESKTOP, name) for name in (
        "SCRNSAVE.EXE", "ScreenSaveActive", "ScreenSaveTimeOut", "ScreenSaverIsSecure",
    )}


def start(path, env, *args):
    return subprocess.Popen([str(path), *args], env=env)


def stop(process):
    if process.poll() is None:
        process.kill()
    process.wait(timeout=15)


def verify_app(app, env, config, version):
    process = start(app, env)
    try:
        hwnd = window(process, "DCTMain")
        assert version.removesuffix(".0") in text(hwnd)
        combo = USER32.GetDlgItem(hwnd, 160)
        assert combo and USER32.SendMessageW(combo, 0x146, 0, 0) == 3
        for index, label in [(1, "1080p"), (2, "4K")]:
            USER32.SendMessageW(combo, 0x14E, index, 0)
            assert text(combo) == label
            command(hwnd, 160, 1)
            command(hwnd, 140)
            wait(lambda: config.is_file() and json.loads(config.read_bytes()).get("FullscreenResolution") == label, label)
            post(hwnd, 0x100, 0x7A)
            fullscreen = window(process, "DCTRender")
            post(fullscreen, 0x100, 0x20)
            post(fullscreen, 0x100, ord("H"))
            time.sleep(0.4)
            post(fullscreen, 0x100, 0x1B)
            wait(lambda: not visible_windows(process, "DCTRender"), f"{label} 전체 화면 종료")
            print(f"{label} 선택, 설정 저장, 전체 화면·단축키 통과", flush=True)
        post(hwnd, 0x10)
        assert process.wait(timeout=12) == 0
    finally:
        stop(process)


def wizard(setup, env, complete=False, saver=False, fail=False):
    process = start(setup, env)
    try:
        hwnd = window(process, "DCTSetup")
        primary = USER32.GetDlgItem(hwnd, 101)
        assert text(primary) == "설치"
        if complete:
            if saver:
                checkbox = USER32.GetDlgItem(hwnd, 102)
                USER32.SendMessageW(checkbox, 0xF1, 1, 0)
            command(hwnd, 101)
            wait(lambda: text(primary) in {"실행", "다시 시도"}, "설치 결과")
            expected = "다시 시도" if fail else "실행"
            assert text(primary) == expected, f"설치 결과가 {expected!r} 상태가 아닙니다."
        command(hwnd, 103)
        assert process.wait(timeout=15) == 0
    finally:
        stop(process)


def dialog(process, expected, response):
    last = ""

    def find():
        nonlocal last
        for hwnd in visible_windows(process, "#32770"):
            content = []

            @ENUM
            def read(child, _):
                value = text(child)
                if value:
                    content.append(value)
                return True

            USER32.EnumChildWindows(hwnd, read, 0)
            last = " ".join(content)
            if expected in last and USER32.GetDlgItem(hwnd, response):
                return hwnd
            if "복원하지 못했어요" in last or "try uninstalling again" in last or "could not be completed" in last:
                raise AssertionError(last)
        return None

    try:
        hwnd = wait(find, expected, 18)
        post(hwnd, 0x111, response)
        wait(lambda: not USER32.IsWindow(hwnd) or process.poll() is not None, expected, 18)
    except AssertionError as error:
        raise AssertionError(f"{error}. 마지막 대화 상자: {last}") from error


def uninstall(exe, env):
    process = start(exe, env, "/uninstall")
    try:
        dialog(process, "Christmas Tree를 삭제할까요?", 6)
        dialog(process, "저장한 설정도 지울까요?", 7)
        dialog(process, "Christmas Tree를 삭제했어요.", 1)
        assert process.wait(timeout=15) == 0
        wait(lambda: not exe.exists(), "제거 프로그램 자체 삭제")
    finally:
        stop(process)


def install_legacy(setup, env):
    process = start(setup, env)
    try:
        dialog(process, "Christmas Tree 2.2을 설치할까요?", 6)
        dialog(process, "화면 보호기로 사용할까요?", 7)
        dialog(process, "설치가 끝났어요.", 7)
        assert process.wait(timeout=15) == 0
    finally:
        stop(process)


def verify_setup(setup, app, ui_only, version, legacy):
    assert SHELL.ExtractIconExW(str(setup), -1, None, None, 0) > 0
    assert SHELL.ExtractIconExW(str(app), -1, None, None, 0) > 0
    print("설치기 및 앱에서 Windows 아이콘 리소스 확인", flush=True)
    with tempfile.TemporaryDirectory(prefix="dct-smoke-") as directory:
        profile = Path(directory)
        local, roaming = profile / "Local", profile / "Roaming"
        env = os.environ | {"LOCALAPPDATA": str(local), "APPDATA": str(roaming)}
        destination = local / "DeveloperChristmasTree"
        config = destination / "settings.json"
        baseline = registry(UNINSTALL, "InstallLocation")
        wizard(setup, env)
        assert registry(UNINSTALL, "InstallLocation") == baseline and not destination.exists()
        print("설치 화면 표시 및 취소 통과", flush=True)
        if ui_only:
            verify_app(app, env, config, version)
            return
        if baseline is not None:
            raise AssertionError("설치 테스트에 기존 설치가 없는 Windows 프로필이 필요합니다.")
        desktop = desktop_values()
        invalid_menu = roaming / "Microsoft"
        invalid_menu.parent.mkdir(parents=True)
        invalid_menu.write_text("시작 메뉴 경로 실패 검증", encoding="utf-8")
        wizard(setup, env, complete=True, fail=True)
        assert not list(destination.glob("*"))
        assert registry(UNINSTALL, "InstallLocation") is None and desktop_values() == desktop
        invalid_menu.unlink()
        print("설치 실패 화면과 부분 설치 롤백 통과", flush=True)
        installed_app = destination / "DeveloperChristmasTree.exe"
        scr = destination / "DeveloperChristmasTree.scr"
        try:
            wizard(setup, env, complete=True, saver=True)
            assert scr.read_bytes() == installed_app.read_bytes()
            assert SHELL.ExtractIconExW(str(scr), -1, None, None, 0) > 0
            assert registry(UNINSTALL, "DisplayVersion")[0] == version
            assert Path(registry(DESKTOP, "SCRNSAVE.EXE")[0]) == scr
            assert registry(DESKTOP, "ScreenSaverIsSecure") == desktop["ScreenSaverIsSecure"]
            print("설치, 버전 등록, 화면 보호기 등록 통과", flush=True)
            verify_app(installed_app, env, config, version)
        finally:
            uninstaller = destination / "Uninstall.exe"
            if uninstaller.exists():
                uninstall(uninstaller, env)
        assert config.exists() and not installed_app.exists() and not scr.exists()
        assert registry(UNINSTALL, "InstallLocation") is None
        assert desktop_values() == desktop
        print("제거, 앱 설정 유지, 기존 화면 보호기 값 복원 통과", flush=True)

        if legacy:
            try:
                install_legacy(legacy, env)
                assert registry(UNINSTALL, "DisplayVersion")[0] == "2.2.0"
                original = b'{"Theme":"Ice Blue","Message":"holiday","FPS":30}\n'
                config.write_bytes(original)
                wizard(setup, env, complete=True)
                assert registry(UNINSTALL, "DisplayVersion")[0] == version
                assert config.read_bytes() == original
                assert installed_app.read_bytes() == app.read_bytes()
                print("v2.2 덮어쓰기 및 기존 설정 바이트 보존 통과", flush=True)
            finally:
                uninstaller = destination / "Uninstall.exe"
                if uninstaller.exists() and registry(UNINSTALL, "DisplayVersion") == (version, winreg.REG_SZ):
                    uninstall(uninstaller, env)


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description="Windows 앱 및 설치기 검증")
    parser.add_argument("setup", type=lambda value: Path(value).resolve())
    parser.add_argument("--app", type=lambda value: Path(value).resolve(), required=True)
    parser.add_argument("--legacy", type=lambda value: Path(value).resolve())
    parser.add_argument("--version", default="2.3.0")
    parser.add_argument("--ui-only", action="store_true")
    args = parser.parse_args()
    verify_setup(args.setup, args.app, args.ui_only, args.version, args.legacy)
