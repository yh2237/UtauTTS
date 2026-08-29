@echo off
setlocal

set "target=%~1"
if not defined target set "target=win"

if /I "%target%"=="win" goto build_win
if /I "%target%"=="linux" goto build_linux
if /I "%target%"=="both" goto build_both
if /I "%target%"=="help" goto usage
if /I "%target%"=="/?" goto usage
if /I "%target%"=="-h" goto usage
if /I "%target%"=="--help" goto usage

echo Unknown build target: %target%
goto usage_error

:build_both
call :build_win
if errorlevel 1 exit /b %errorlevel%
call :build_linux
exit /b %errorlevel%

:build_win
echo === Building Windows package ===
powershell -NoProfile -ExecutionPolicy Bypass -File "%~dp0tools\build-release.ps1"
exit /b %errorlevel%

:build_linux
echo === Building Linux package through WSL ===
where wsl.exe >nul 2>&1
if errorlevel 1 (
    echo WSL is required for the Linux build. Install WSL and a Linux distribution first.
    exit /b 1
)

set "wsl_root="
if defined UTAUTTS_WSL_DISTRO (
    for /f "usebackq delims=" %%I in (`wsl.exe -d "%UTAUTTS_WSL_DISTRO%" wslpath -a "%~dp0." 2^>nul`) do set "wsl_root=%%I"
) else (
    for /f "usebackq delims=" %%I in (`wsl.exe wslpath -a "%~dp0." 2^>nul`) do set "wsl_root=%%I"
)
if not defined wsl_root (
    echo WSL could not translate the project path. Ensure a default Linux distribution is installed.
    exit /b 1
)

if defined UTAUTTS_WSL_DISTRO (
    wsl.exe -d "%UTAUTTS_WSL_DISTRO%" --cd "%wsl_root%" -- bash -lc "exec bash ./tools/build-linux.sh"
) else (
    wsl.exe --cd "%wsl_root%" -- bash -lc "exec bash ./tools/build-linux.sh"
)
exit /b %errorlevel%

:usage
echo Usage: build.bat [win^|linux^|both]
echo.
echo   win    Build the Windows release package. (default)
echo   linux  Build the Linux package through WSL.
echo   both   Build Windows and then Linux packages.
echo.
echo Set UTAUTTS_WSL_DISTRO to select a non-default WSL distribution.
exit /b 0

:usage_error
echo.
call :usage
exit /b 2
