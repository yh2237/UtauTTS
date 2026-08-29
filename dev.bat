@echo off
cd /d "%~dp0"

set "voice_dir=%UTAUTTS_VOICE_DIR%"
if not defined voice_dir set "voice_dir=%~dp0sample"

echo === UtauTTS Dev Server ===
echo.
echo Open http://127.0.0.1:8080
echo.

go run ./cmd/utautts-server --voice-dir "%voice_dir%" %*
