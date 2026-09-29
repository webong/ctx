@echo off
rem ctx native Windows shim for Claude Code
"%~dp0ctx.exe" run claude %*
exit /b %ERRORLEVEL%
