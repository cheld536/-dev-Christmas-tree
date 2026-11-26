import argparse
import ctypes
from ctypes import wintypes
from datetime import date, timedelta
import json
import os
from pathlib import Path
import struct
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
USER32.GetDlgCtrlID.argtypes = [wintypes.HWND]
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


def set_text(hwnd, value):
    buffer = ctypes.create_unicode_buffer(value)
    assert USER32.SendMessageW(hwnd, 0xC, 0, ctypes.addressof(buffer))


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


def verify_app(app, env, config, version, check_lock_screen=False):
    process = start(app, env)
    try:
        hwnd = window(process, "DCTMain")
        assert text(hwnd) == "Developer Christmas Tree"
        assert text(USER32.GetDlgItem(hwnd, 161)) == "v" + version
        lock_control = USER32.GetDlgItem(hwnd, 176)
        assert text(lock_control) == "잠금 화면 이미지 적용"
        if check_lock_screen:
            command(hwnd, 176)
            applied = "잠금 화면에 적용됨"
            wait(lambda: text(lock_control) == applied or visible_windows(process, "#32770"),
                 "잠금 화면 이미지 적용 요청 완료")
            if text(lock_control) != applied:
                dialog(process, "이미지를 저장했습니다.", 2)
                print("자동 적용을 지원하지 않는 환경의 사진 선택 안내 통과", flush=True)
            else:
                print("Windows 잠금 화면 이미지 적용 API 성공", flush=True)
            saved, = (config.parent / "lock-screen").glob("Christmas-Tree-*.png")
            header = saved.read_bytes()[:24]
            assert header.startswith(b"\x89PNG\r\n\x1a\n")
            assert struct.unpack(">II", header[16:24]) == (1920, 1080)
            print("잠금 화면용 1080p PNG 저장 통과", flush=True)
        command(hwnd, 162)
        assert text(USER32.GetDlgItem(hwnd, 162)) == "커플 기념일"
        fields = {key: USER32.GetDlgItem(hwnd, key) for key in (164, 165, 166, 167)}
        wait(lambda: USER32.IsWindowVisible(fields[164]), "커플 기념일 탭")
        command(hwnd, 163)
        wait(lambda: text(fields[167]) == "두 이름을 모두 입력하세요.", "이름 필수 안내")
        set_text(fields[164], "지우")
        set_text(fields[165], "하늘")
        set_text(fields[166], "2999-01-01")
        wait(lambda: "오늘이나 이전 날짜" in text(fields[167]), "미래 시작일 안내")
        today = date.today().isoformat()
        set_text(fields[166], today)
        calendar = USER32.GetDlgItem(hwnd, 169)
        assert text(USER32.GetDlgItem(hwnd, 168)) == "달력에서 선택"
        assert not USER32.IsWindowVisible(calendar)
        command(hwnd, 168)
        wait(lambda: USER32.IsWindowVisible(calendar), "시작일 달력 열기")
        post(calendar, 0x100, 0x25)
        post(calendar, 0x100, 0x1B)
        wait(lambda: not USER32.IsWindowVisible(calendar), "달력 취소")
        assert text(fields[166]) == today
        command(hwnd, 168)
        wait(lambda: USER32.IsWindowVisible(calendar), "시작일 달력 다시 열기")
        post(calendar, 0x100, 0x25)
        post(calendar, 0x100, 0x0D)
        yesterday = (date.today() - timedelta(days=1)).isoformat()
        wait(lambda: not USER32.IsWindowVisible(calendar) and text(fields[166]) == yesterday, "달력에서 전날 선택")
        wait(lambda: text(fields[167]) == "지우 ♥️ 하늘  D+2일", "달력 선택 후 D+2일")
        set_text(fields[166], "")
        command(hwnd, 168)
        wait(lambda: USER32.IsWindowVisible(calendar), "빈 시작일에서 달력 열기")
        post(calendar, 0x100, 0x27)
        post(calendar, 0x100, 0x0D)
        wait(lambda: not USER32.IsWindowVisible(calendar) and text(fields[166]) == today, "달력에서 미래 날짜 제한 및 오늘 선택")
        print("시작일 달력 열기, Esc 취소, 날짜 선택 및 D+ 계산 통과", flush=True)
        expected_caption = "지우 ♥️ 하늘  D+1일"
        wait(lambda: text(fields[167]) == expected_caption, "한글 이름과 D+1일")
        command(hwnd, 140)
        expected = {"Enabled": True, "FirstName": "지우", "SecondName": "하늘", "StartDate": today}
        wait(lambda: config.is_file() and json.loads(config.read_bytes()).get("Couple") == expected, "커플 기념일 저장")
        print("커플 기념일 입력 검증, 한글 출력 및 설정 저장 통과", flush=True)
        command(hwnd, 148)
        story_status = USER32.GetDlgItem(hwnd, 174)
        wait(lambda: USER32.IsWindowVisible(story_status), "음악과 효과 탭의 성탄 이야기 설정")
        assert json.loads(config.read_bytes())["Stories"] is True
        command(hwnd, 172)
        command(hwnd, 140)
        wait(lambda: json.loads(config.read_bytes())["Stories"] is False, "성탄 이야기 자동 재생 끄기")
        for scene in ("산타와 루돌프", "트리 아래 선물", "성냥불과 작은 손님", "눈사람의 목도리", "겨울 하늘의 별똥별", "산타와 루돌프"):
            command(hwnd, 173)
            wait(lambda: text(story_status) == scene, f"이야기 미리보기 {scene}")
        print("성탄 이야기 다섯 장면 전환, 순환 및 자동 재생 끄기 통과", flush=True)
        def equalizers():
            settings = json.loads(config.read_bytes())
            return settings["Spectrum"], settings["RingSpectrum"]

        assert text(USER32.GetDlgItem(hwnd, 155)) == "하단 막대 이퀄라이저"
        assert text(USER32.GetDlgItem(hwnd, 175)) == "트리 둘레 이퀄라이저"
        assert equalizers() == (True, False)
        for control, expected_modes in [(175, (False, True)), (155, (True, False)),
                                        (155, (False, False)), (175, (False, True))]:
            command(hwnd, control)
            command(hwnd, 140)
            wait(lambda: equalizers() == expected_modes, f"이퀄라이저 전환 및 저장 {expected_modes}")
        post(hwnd, 0x100, 0x7A)
        fullscreen = window(process, "DCTRender")
        post(fullscreen, 0x100, ord("V"))
        post(fullscreen, 0x100, 0x1B)
        wait(lambda: not visible_windows(process, "DCTRender"), "하단 막대 전환 후 전체 화면 종료")
        command(hwnd, 140)
        wait(lambda: equalizers() == (True, False), "전체 화면 V로 하단 막대를 켜면 둘레 막대 끄기")
        command(hwnd, 175)
        command(hwnd, 140)
        wait(lambda: equalizers() == (False, True), "트리 둘레 이퀄라이저를 다시 선택")
        print("이퀄라이저 동시 표시 방지, 둘 다 끄기, 전체 화면 V 키 및 저장 통과", flush=True)
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

    process = start(app, env)
    try:
        hwnd = window(process, "DCTMain")
        command(hwnd, 140)
        assert json.loads(config.read_bytes())["Stories"] is False
        assert equalizers() == (False, True)
        command(hwnd, 162)
        status = USER32.GetDlgItem(hwnd, 167)
        wait(lambda: USER32.IsWindowVisible(status) and text(status) == expected_caption, "재실행 후 기념일 유지")
        command(hwnd, 163)
        wait(lambda: text(status).startswith("기념일을 켜면"), "기념일 끄기")
        post(hwnd, 0x10)
        assert process.wait(timeout=12) == 0
        print("재실행 후 커플 기념일 복원 및 끄기 통과", flush=True)
    finally:
        stop(process)


def wizard(setup, env, version, complete=False, saver=False, fail=False):
    process = start(setup, env)
    try:
        hwnd = window(process, "DCTSetup")
        assert version not in text(hwnd)
        assert text(USER32.GetDlgItem(hwnd, 104)) == "v" + version
        desktop_option = USER32.GetDlgItem(hwnd, 105)
        assert text(desktop_option) == "바탕 화면에 바로가기 만들기"
        assert USER32.SendMessageW(desktop_option, 0xF0, 0, 0) == 1
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
            buttons = []

            @ENUM
            def read(child, _):
                value = text(child)
                if value:
                    content.append(value)
                klass = ctypes.create_unicode_buffer(30)
                USER32.GetClassNameW(child, klass, len(klass))
                if klass.value == "Button":
                    buttons.append(child)
                return True

            USER32.EnumChildWindows(hwnd, read, 0)
            last = " ".join(content)
            if expected in last:
                button = USER32.GetDlgItem(hwnd, response)
                if response == 1 and len(buttons) == 1:
                    button = buttons[0]
                if button:
                    return hwnd, button
            if "복원하지 못했어요" in last or "try uninstalling again" in last or "could not be completed" in last:
                raise AssertionError(last)
        return None

    try:
        hwnd, button = wait(find, expected, 18)
        button_id = USER32.GetDlgCtrlID(button)
        if not USER32.PostMessageW(hwnd, 0x111, button_id, button):
            raise ctypes.WinError(ctypes.get_last_error())
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


def verify_setup(setup, app, ui_only, version, legacy, previous):
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
        wizard(setup, env, version)
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
        wizard(setup, env, version, complete=True, fail=True)
        assert not list(destination.glob("*"))
        assert registry(UNINSTALL, "InstallLocation") is None and desktop_values() == desktop
        invalid_menu.unlink()
        print("설치 실패 화면과 부분 설치 롤백 통과", flush=True)
        installed_app = destination / "DeveloperChristmasTree.exe"
        scr = destination / "DeveloperChristmasTree.scr"
        try:
            wizard(setup, env, version, complete=True, saver=True)
            assert scr.read_bytes() == installed_app.read_bytes()
            assert SHELL.ExtractIconExW(str(scr), -1, None, None, 0) > 0
            assert registry(UNINSTALL, "DisplayVersion")[0] == version
            shortcut = Path(registry(UNINSTALL, "DesktopShortcut")[0])
            assert shortcut.name == "Developer Christmas Tree.lnk" and shortcut.is_file()
            assert Path(registry(DESKTOP, "SCRNSAVE.EXE")[0]) == scr
            assert registry(DESKTOP, "ScreenSaverIsSecure") == desktop["ScreenSaverIsSecure"]
            print("설치, 버전 등록, 화면 보호기 등록 통과", flush=True)
            verify_app(installed_app, env, config, version, check_lock_screen=True)
        finally:
            uninstaller = destination / "Uninstall.exe"
            if uninstaller.exists():
                uninstall(uninstaller, env)
        assert config.exists() and not installed_app.exists() and not scr.exists() and not shortcut.exists()
        assert registry(UNINSTALL, "InstallLocation") is None
        assert desktop_values() == desktop
        print("제거, 앱 설정 유지, 기존 화면 보호기 값 복원 통과", flush=True)

        for old_setup, old_version in ((legacy, "2.2.0"), (previous, "2.3.0")):
            if not old_setup:
                continue
            try:
                if old_version == "2.2.0":
                    install_legacy(old_setup, env)
                else:
                    wizard(old_setup, env, old_version, complete=True)
                assert registry(UNINSTALL, "DisplayVersion")[0] == old_version
                original = b'{"Theme":"Ice Blue","Message":"holiday","FPS":30}\n'
                config.write_bytes(original)
                wizard(setup, env, version, complete=True)
                assert registry(UNINSTALL, "DisplayVersion")[0] == version
                assert config.read_bytes() == original
                assert installed_app.read_bytes() == app.read_bytes()
                print(f"v{old_version} 덮어쓰기 및 기존 설정 바이트 보존 통과", flush=True)
            finally:
                uninstaller = destination / "Uninstall.exe"
                if uninstaller.exists() and registry(UNINSTALL, "DisplayVersion") == (version, winreg.REG_SZ):
                    uninstall(uninstaller, env)


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description="Windows 앱 및 설치기 검증")
    parser.add_argument("setup", type=lambda value: Path(value).resolve())
    parser.add_argument("--app", type=lambda value: Path(value).resolve(), required=True)
    parser.add_argument("--legacy", type=lambda value: Path(value).resolve())
    parser.add_argument("--previous", type=lambda value: Path(value).resolve())
    parser.add_argument("--version", default="2.3.0")
    parser.add_argument("--ui-only", action="store_true")
    args = parser.parse_args()
    verify_setup(args.setup, args.app, args.ui_only, args.version, args.legacy, args.previous)
