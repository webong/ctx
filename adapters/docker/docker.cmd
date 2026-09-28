@echo off
rem ctx native Windows shim for Docker
"%~dp0ctx.exe" run docker %*
exit /b %ERRORLEVEL%
