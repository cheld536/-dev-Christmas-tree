@echo off
setlocal
cd /d "%~dp0"
if not exist "dist\DeveloperChristmasTree.scr" (
  echo Run BUILD_WINDOWS.bat first.
  pause
  exit /b 1
)
set "TARGET=%LOCALAPPDATA%\DeveloperChristmasTree"
if not exist "%TARGET%" mkdir "%TARGET%"
copy /y "dist\DeveloperChristmasTree.scr" "%TARGET%\DeveloperChristmasTree.scr" >nul
if errorlevel 1 (
  echo Copy failed.
  pause
  exit /b 1
)
rundll32.exe desk.cpl,InstallScreenSaver "%TARGET%\DeveloperChristmasTree.scr"
echo Select the screen saver, set the wait time, and click Apply.
pause
