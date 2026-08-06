@echo off
cd /d "%~dp0"
powershell -NoProfile -ExecutionPolicy Bypass -File "CacheCleaner.ps1"
pause