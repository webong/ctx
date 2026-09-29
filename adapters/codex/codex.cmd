@echo off
rem ctx native Windows shim for Codex CLI
"%~dp0ctx.exe" run codex %*
exit /b %ERRORLEVEL%
