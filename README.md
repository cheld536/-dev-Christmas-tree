# Developer Christmas Tree

Windows용 3D ASCII 크리스마스 트리, 화면 보호기, 음악 시각화 프로그램입니다. Python 터미널 프로토타입을 시작으로 현재는 Go/Win32 네이티브 앱으로 개발하고 있습니다.

**현재 버전: v2.2.0 (테스트 빌드)** · Windows 10/11 x64 대상. Python이나 Go를 설치할 필요 없는 단일 Setup.exe를 배포합니다. 개발자만 Go가 필요합니다.

[버전별 설치 파일과 SHA256 검사값](https://github.com/cheld536/-dev-Christmas-tree/releases)을 받을 수 있습니다. v1.0.0에는 설치 과정 없이 실행하는 Python/Tkinter EXE가 제공됩니다.

## 사용법

1. 설치 파일을 실행합니다. 기존 버전이 실행 중이면 먼저 종료합니다.
2. 설정 창에서 테마, 회전, 조명, 코드 스타일, 음악 반응을 조절하고 저장합니다.
3. F11로 일반 전체 화면을 열거나 화면 보호기 미리보기를 실행합니다. 화면 보호기는 입력 시 종료되며, 일반 전체 화면은 Esc/F11로 종료됩니다.
4. 시스템 음악을 재생하면 WASAPI로 분석한 스펙트럼이 표시됩니다. Windows 미디어 세션이 메타데이터를 제공하면 곡명·아티스트·진행률·커버를 표시합니다. 미제공 시 소리 감지 상태만 표시합니다.

## 단축키 (일반 전체 화면)

| 키 | 기능 |
|---|---|
| F11 / Esc | 돌아가기 |
| Space | 회전 일시 정지 / 재개 |
| ↑ / ↓ | 회전 속도 |
| ← / → | 회전 방향 |
| C / D | 색상 / 코드 스타일 |
| A / P | 음악 반응 / 음악 프리셋 |
| S / M | 눈 / 코드 비 |
| B / V | 빛 번짐 / 이퀄라이저 |
| G / J | 음악 반응 강도 |
| Z / X / N | 이전 곡 / 재생·일시 정지 / 다음 곡 (Windows 미디어 키) |
| Tab | 시스템 사용량 |
| H | 단축키 패널 |

미디어 키의 실제 작동 여부는 재생 앱과 Windows 세션에 의존합니다.

## 빌드 (Windows PowerShell)

Go 1.23 이상 권장. 외부 Go 모듈 의존성 없음.

```powershell
.\build.ps1
```

빌드 결과는 `dist/DeveloperChristmasTree_v2.2_Setup.exe`와 `dist/v2.2.0-SHA256SUMS.txt`에 생성됩니다. 설치기는 먼저 빌드한 `setup/app.exe`를 포함합니다.

GitHub Actions는 `main` 푸시와 풀 리퀘스트에서 Go 및 Python 테스트와 Windows 빌드를 실행합니다. `main`에 포함된 커밋의 새 `vMAJOR.MINOR.PATCH` 태그를 푸시하면 해당 버전으로 설치기를 빌드하고, EXE와 검사값을 검증한 뒤 GitHub Release를 공개합니다. 복원 버전의 [배포 검사값](docs/release-checksums.txt)은 별도로 기록했습니다.

## 구성

```text
engine.go            3D 좌표·투영·깊이·테마·파티클
engine_test.go       그래픽 엔진 및 설정 테스트
audio.go             FFT·비트·BPM·반응 프리셋
audio_test.go        합성 신호 테스트
extras_windows.go    WASAPI loopback 및 리소스 모니터링
media_windows.go     GSMTC 미디어 메타데이터 조회/커버 디코딩
overlay_windows.go   음악 카드, 코드, 단축키, 알림
render_windows.go    렌더링 및 GDI 백버퍼 (레거시 GL 코드 보관)
win_windows.go       Win32 창·설정·전체 화면·화면 보호기
setup/               설치 및 제거 프로그램
legacy/python/       초기 Python/Tkinter 프로토타입
docs/                개발 기록, 구조, 테스트 체크리스트
.github/workflows/   Windows CI 빌드
```

설정 경로: `%LOCALAPPDATA%\DeveloperChristmasTree\settings.json`. 기존 v2.0/v2.1 설정을 읽고 유지합니다(단, v2.2는 GDI 고정으로 렌더러 설정을 변경합니다). Windows 설정의 설치된 앱에서 제거할 수 있으며 저장된 설정을 삭제할지 선택합니다. 기존 화면 보호기 복원 로직이 포함되어 있습니다. 화면 보호기 자체는 PC 잠금 기능이 아니므로 Windows 로그온 화면 표시 설정을 별도로 관리하세요.

## 검증 상태

Windows 11 x64에서 v1 설정 창 실행, v2 세 버전의 설치·앱 실행·제거, 버전 간 덮어쓰기와 설정 유지 여부를 확인했습니다. WASAPI·GSMTC·다중 모니터·DPI 및 실제 프레임 성능은 추가 검증 대상입니다. 배포 EXE에는 코드 서명이 없습니다. 음악 분석과 메타데이터 조회는 로컬에서 수행하며 자체 서버로 전송하지 않습니다.

개발 이력은 [CHANGELOG](CHANGELOG.md), 설계는 [architecture](docs/ARCHITECTURE.md), 실제 환경 검증 항목은 [Windows QA](docs/WINDOWS_QA.md)를 참고하세요.

Git 이력은 제공된 과거 소스 스냅샷을 바탕으로 재구성했습니다. 기존에 실제로 존재하던 커밋 이력이라는 의미는 아닙니다. 라이선스는 저장소 소유자가 결정하기 전까지 미지정 상태입니다.
