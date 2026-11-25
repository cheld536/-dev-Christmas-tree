# Developer Christmas Tree — Windows Screen Saver

기존 Python 터미널 트리의 3D 점군, 원근 투영, 깊이 버퍼 및 ASCII 장식을 Tkinter GUI로 옮긴 화면 보호기입니다.

## Windows 10/11 사용법

1. Python 3.10 이상(64비트 Windows라면 64비트 Python, Tcl/Tk 포함)을 설치합니다. 기존 Python 설치가 있다면 `python -m tkinter`로 GUI 지원을 확인합니다.
2. ZIP을 폴더에 압축 해제합니다.
3. `BUILD_WINDOWS.bat`을 더블클릭합니다. 빌드 PC의 인터넷 연결이 필요합니다(PyInstaller 설치). 프로젝트 내부 `.venv`만 사용합니다.
4. `dist/DeveloperChristmasTree.exe`가 설정 앱, `dist/DeveloperChristmasTree.scr`가 Windows 화면 보호기입니다. 생성된 파일은 Python을 별도 설치하지 않은 같은 아키텍처의 Windows에서도 실행하도록 패키징됩니다.
5. `INSTALL_SCREENSAVER.bat`을 실행하면 SCR을 `%LOCALAPPDATA%\DeveloperChristmasTree`로 복사하고 Windows의 화면 보호기 설정을 엽니다. 화면 보호기를 선택한 후 대기 시간을 설정하고 적용합니다. 관리자 권한은 일반적으로 필요하지 않지만 회사 PC의 정책에 의해 차단될 수 있습니다.

화면 보호기 사용 시 Windows의 '다시 시작할 때 로그온 화면 표시' 옵션을 사용해야 잠금 기능이 적용됩니다. 이 앱은 잠금 기능을 직접 구현하지 않습니다.

## 빌드 없이 실행

```powershell
python christmas_screensaver.py
python christmas_screensaver.py /s
```

## 옵션

- 7개 색상 테마: 무지개 RGB, 클래식, 아이스 블루, 사이버펑크, 골드 & 화이트, 매트릭스 그린, 사용자 지정
- 트리/장식 색상 및 배경색 직접 선택
- 회전 방향(시계/반시계/정지), 회전 속도와 조명 변화 속도(0~4배), 트리 크기(45~100%)
- 목표 FPS(10~60), 눈, 개발자 코드 장식, 메시지 ON/OFF
- 실시간 미리보기, 전체 화면 테스트, 기본값 복원, 설정 저장
- 실제 FPS는 CPU, 해상도, Tk 렌더링 성능에 따라 달라집니다.

설정은 `%LOCALAPPDATA%\DeveloperChristmasTree\settings.json`에 저장됩니다. 저장 버튼을 눌러야 다음 실행에도 반영됩니다. 화면 보호기는 키 입력, 마우스 클릭 또는 마우스 이동(시작 후 0.75초 이후 감지)으로 종료됩니다. 기본 구현은 주 모니터 한 대 기준입니다.

## Windows 실행 모드

- `.exe` 단독 실행 또는 `/c`: 설정 UI
- `.scr` 단독 실행 또는 `/s`: 전체 화면
- `/p <HWND>` 혹은 `/p:<HWND>`: Windows 설정 창 내 미리보기(Win32 SetParent 사용)

## 검증 및 제한

Python 문법, 설정 저장/로드와 값 검증, 7개 테마의 3D 회전 렌더링, 명령행 모드 파싱, Tk GUI/전체 화면 복귀를 Linux Xvfb 환경에서 테스트했습니다. Windows 실제 SCR 등록, HWND 임베딩, PyInstaller EXE 생성 및 실행은 Windows 환경에서 추가 확인해야 합니다. Windows용 바이너리는 이 ZIP에 포함되어 있지 않습니다. 빌드 결과는 코드 서명되지 않으므로 Windows SmartScreen 또는 회사 정책에 의해 경고/차단될 수 있습니다. 보안 정책을 우회하지 마세요.

```powershell
python test_app.py
```

## 제거

Windows 화면 보호기 설정에서 다른 화면 보호기 또는 '없음'을 선택하고 적용한 뒤 `%LOCALAPPDATA%\DeveloperChristmasTree` 폴더를 삭제합니다. 프로젝트 폴더는 별도로 삭제할 수 있습니다.
