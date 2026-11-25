@echo off
setlocal
cd /d "%~dp0"
set "PYTHONUTF8=1"
where py >nul 2>&1
if errorlevel 1 goto use_python
py -3 -m venv .venv
if errorlevel 1 goto fail
goto build
:use_python
python -m venv .venv
if errorlevel 1 goto fail
:build
".venv\Scripts\python.exe" -m pip install --disable-pip-version-check pyinstaller
if errorlevel 1 goto fail
".venv\Scripts\python.exe" -m PyInstaller --noconfirm --clean --onefile --windowed --name DeveloperChristmasTree christmas_screensaver.py
if errorlevel 1 goto fail
copy /y "dist\DeveloperChristmasTree.exe" "dist\DeveloperChristmasTree.scr" >nul
if errorlevel 1 goto fail
echo.
echo Build complete: dist\DeveloperChristmasTree.exe and .scr
start "" "%~dp0dist"
pause
exit /b 0
:fail
echo Build failed. Install Python 3 with Tcl/Tk, check internet access and retry.
pause
exit /b 1
