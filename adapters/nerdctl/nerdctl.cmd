@echo off
rem ctx native Windows shim for nerdctl/containerd
"%~dp0ctx.exe" run nerdctl %*
exit /b %ERRORLEVEL%
