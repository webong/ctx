[CmdletBinding()]
param(
    [string]$BinDir = $(if ($env:CTX_BIN_DIR) { $env:CTX_BIN_DIR } else { Join-Path $env:LOCALAPPDATA 'Programs\ctx\bin' }),
    [string]$ConfigDir = $(if ($env:CTX_HOME) { $env:CTX_HOME } else { Join-Path $env:APPDATA 'ctx' }),
    [string]$Version = 'latest'
)

$ErrorActionPreference = 'Stop'
$repositoryRoot = $PSScriptRoot
$localSource = Test-Path (Join-Path $repositoryRoot 'cmd\ctx\main.go')

New-Item -ItemType Directory -Force -Path $BinDir | Out-Null
New-Item -ItemType Directory -Force -Path $ConfigDir | Out-Null

$ctxTarget = Join-Path $BinDir 'ctx.exe'
if ($localSource) {
    if (-not (Get-Command go -ErrorAction SilentlyContinue)) {
        throw 'Go is required when installing ctx from a source checkout.'
    }
    Push-Location $repositoryRoot
    try {
        & go build -o $ctxTarget ./cmd/ctx
        if ($LASTEXITCODE -ne 0) { throw 'go build failed' }
    }
    finally {
        Pop-Location
    }
}
else {
    $architecture = switch ([System.Runtime.InteropServices.RuntimeInformation]::OSArchitecture.ToString().ToLowerInvariant()) {
        'x64' { 'amd64' }
        'arm64' { 'arm64' }
        default { throw "Unsupported Windows architecture: $_" }
    }
    $release = if ($Version -eq 'latest') { 'latest/download' } else { "download/$Version" }
    $url = "https://github.com/webong/ctx/releases/$release/ctx-windows-$architecture.exe"
    Invoke-WebRequest -UseBasicParsing -Uri $url -OutFile $ctxTarget
}

foreach ($engine in @('docker', 'podman', 'nerdctl')) {
    $shim = "@echo off`r`nrem ctx native Windows shim for $engine`r`n`"%~dp0ctx.exe`" run $engine %*`r`nexit /b %ERRORLEVEL%`r`n"
    Set-Content -Encoding ASCII -Path (Join-Path $BinDir "$engine.cmd") -Value $shim -NoNewline
}

$configFile = Join-Path $ConfigDir 'config.toml'
if (-not (Test-Path $configFile)) {
    @'
# ctx native configuration
# docker_default = "desktop-linux"
# podman_default = "podman-machine-default"
# nerdctl_default = "default"
'@ | Set-Content -Encoding UTF8 $configFile
}

Write-Host "Installed ctx.exe and container shims in $BinDir"
Write-Host "Config: $configFile"
if (($env:PATH -split ';') -notcontains $BinDir) {
    Write-Host "Add this directory before Docker, Podman, and nerdctl on PATH: $BinDir"
}
Write-Warning 'Native Windows support is a migration preview; adapter, browser, transfer, and mutation commands are not at parity yet.'
