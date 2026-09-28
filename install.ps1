[CmdletBinding()]
param(
    [string]$BinDir = $(if ($env:CTX_BIN_DIR) { $env:CTX_BIN_DIR } else { Join-Path $env:LOCALAPPDATA 'Programs\ctx\bin' }),
    [string]$ConfigDir = $(if ($env:CTX_HOME) { $env:CTX_HOME } else { Join-Path $env:APPDATA 'ctx' }),
    [string]$Version = 'latest'
)

$ErrorActionPreference = 'Stop'
$repositoryRoot = $PSScriptRoot
$localSource = Test-Path (Join-Path $repositoryRoot 'cmd\ctx\main.go')
$firstPartyAdapters = @('firefox', 'chrome', 'chromium', 'safari', 'kube', 'aws', 'gcloud', 'postgres', 'mysql')

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

$adaptersRoot = Join-Path $ConfigDir 'adapters'
New-Item -ItemType Directory -Force -Path $adaptersRoot | Out-Null
foreach ($adapter in $firstPartyAdapters) {
    $target = Join-Path $adaptersRoot $adapter
    if (Test-Path $target) {
        $targetManifest = Join-Path $target 'adapter.toml'
        if (-not (Test-Path $targetManifest) -or -not (Select-String -Quiet -Path $targetManifest -Pattern '^first_party\s*=\s*"true"\s*$')) {
            throw "Refusing to replace non-first-party adapter: $target"
        }
        Remove-Item -Recurse -Force $target
    }
    if ($localSource) {
        Copy-Item -Recurse -Path (Join-Path $repositoryRoot "adapters\$adapter") -Destination $target
    }
    else {
        New-Item -ItemType Directory -Force -Path $target | Out-Null
        $sourceRef = if ($Version -eq 'latest') { 'main' } else { $Version }
        $sourceBase = "https://raw.githubusercontent.com/webong/ctx/$sourceRef/adapters/$adapter"
        $manifestPath = Join-Path $target 'adapter.toml'
        Invoke-WebRequest -UseBasicParsing -Uri "$sourceBase/adapter.toml" -OutFile $manifestPath
        $manifest = Get-Content -Raw $manifestPath
        foreach ($key in @('executable', 'executable_windows')) {
            $match = [regex]::Match($manifest, "(?m)^$key\s*=\s*`"([^`"]+)`"\s*$")
            if ($match.Success) {
                $executable = $match.Groups[1].Value
                Invoke-WebRequest -UseBasicParsing -Uri "$sourceBase/$executable" -OutFile (Join-Path $target $executable)
            }
        }
    }
}

$previousCtxHome = $env:CTX_HOME
try {
    $env:CTX_HOME = $ConfigDir
    foreach ($adapter in $firstPartyAdapters) {
        & $ctxTarget adapter trust $adapter | Out-Null
        if ($LASTEXITCODE -ne 0) { throw "Failed to trust bundled adapter: $adapter" }
    }
}
finally {
    $env:CTX_HOME = $previousCtxHome
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

Write-Host "Installed ctx.exe, container shims, and first-party adapters in $BinDir"
Write-Host "Config: $configFile"
if (($env:PATH -split ';') -notcontains $BinDir) {
    Write-Host "Add this directory before Docker, Podman, and nerdctl on PATH: $BinDir"
}
Write-Warning 'Native Windows support is a migration preview; shell integration and release packaging are still being completed.'
