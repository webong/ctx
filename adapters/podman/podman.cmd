@echo off
rem ctx native Windows shim for Podman
"%~dp0ctx.exe" run podman %*
exit /b %ERRORLEVEL%
