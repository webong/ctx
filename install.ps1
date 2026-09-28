[CmdletBinding()]
param(
    [string]$BinDir = $(if ($env:CTX_BIN_DIR) { $env:CTX_BIN_DIR } else { Join-Path $env:LOCALAPPDATA 'Programs\ctx\bin' }),
    [string]$ConfigDir = $(if ($env:CTX_HOME) { $env:CTX_HOME } else { Join-Path $env:APPDATA 'ctx' }),
    [string]$Version = 'latest',
    [string]$Adapters,
    [switch]$AllAdapters,
    [switch]$Minimal,
    [switch]$Interactive
)

$ErrorActionPreference = 'Stop'
$repositoryRoot = $PSScriptRoot
$localSource = Test-Path (Join-Path $repositoryRoot 'cmd\ctx\main.go')
$bundledAdapters = @('docker', 'podman', 'nerdctl', 'apple', 'firefox', 'chrome', 'chromium', 'safari', 'kube', 'aws', 'gcloud', 'postgres', 'mysql')
$bundleRoot = $null
$downloadRoot = $null

$selectionModes = @($AllAdapters.IsPresent, $Minimal.IsPresent, $Interactive.IsPresent, [bool]$Adapters) | Where-Object { $_ }
if ($selectionModes.Count -gt 1) { throw 'Choose only one of -Adapters, -AllAdapters, -Minimal, or -Interactive.' }

New-Item -ItemType Directory -Force -Path $BinDir | Out-Null
New-Item -ItemType Directory -Force -Path $ConfigDir | Out-Null

$ctxTarget = Join-Path $BinDir 'ctx.exe'
if (Test-Path $ctxTarget) {
    $owned = $false
    try {
        $installedVersion = @(& $ctxTarget version 2>$null) -join "`n"
        $owned = $LASTEXITCODE -eq 0 -and $installedVersion -match '^ctx '
    }
    catch { $owned = $false }
    if (-not $owned) { throw "$ctxTarget exists; choose another BinDir" }
}
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
    $releaseBase = "https://github.com/webong/ctx/releases/$release"
    $asset = "ctx-windows-$architecture.zip"
    $downloadRoot = Join-Path ([System.IO.Path]::GetTempPath()) ("ctx-install-" + [guid]::NewGuid().ToString('N'))
    New-Item -ItemType Directory -Force -Path $downloadRoot | Out-Null
    $archive = Join-Path $downloadRoot $asset
    $checksums = Join-Path $downloadRoot 'checksums.txt'
    Invoke-WebRequest -UseBasicParsing -Uri "$releaseBase/$asset" -OutFile $archive
    Invoke-WebRequest -UseBasicParsing -Uri "$releaseBase/checksums.txt" -OutFile $checksums
    $escapedAsset = [regex]::Escape($asset)
    $checksumLine = Get-Content $checksums | Where-Object { $_ -match "\s\*?$escapedAsset`$" } | Select-Object -First 1
    if (-not $checksumLine) { throw "Release checksum is missing for $asset" }
    $expected = ($checksumLine -split '\s+')[0].ToLowerInvariant()
    $actual = (Get-FileHash -Algorithm SHA256 $archive).Hash.ToLowerInvariant()
    if ($actual -ne $expected) { throw "Release checksum verification failed for $asset" }
    Expand-Archive -Path $archive -DestinationPath $downloadRoot
    $bundleRoot = Join-Path $downloadRoot 'ctx'
    Copy-Item (Join-Path $bundleRoot 'bin\ctx.exe') $ctxTarget
}

$adaptersRoot = Join-Path $ConfigDir 'adapters'
$catalogRoot = Join-Path $ConfigDir 'catalog\adapters'
New-Item -ItemType Directory -Force -Path $adaptersRoot, $catalogRoot | Out-Null
foreach ($adapter in $bundledAdapters) {
    $target = Join-Path $catalogRoot $adapter
    if (Test-Path $target) {
        Remove-Item -Recurse -Force $target
    }
    $adapterSource = if ($bundleRoot) { Join-Path $bundleRoot "adapters\$adapter" } else { Join-Path $repositoryRoot "adapters\$adapter" }
    Copy-Item -Recurse -Path $adapterSource -Destination $target
}

$previousCtxHome = $env:CTX_HOME
$previousBinDir = $env:CTX_BIN_DIR
try {
    $env:CTX_HOME = $ConfigDir
    $env:CTX_BIN_DIR = $BinDir
    & $ctxTarget adapter refresh | Out-Null
    if ($LASTEXITCODE -ne 0) { throw 'Failed to refresh installed adapters.' }
    if ($AllAdapters) { & $ctxTarget setup --all }
    elseif ($Minimal) { & $ctxTarget setup --minimal }
    elseif ($Adapters) { & $ctxTarget setup --adapters $Adapters }
    elseif ($Interactive) { & $ctxTarget setup }
    else {
        Write-Host 'No new adapters selected. Run ctx setup, or reinstall with -Interactive or -Adapters.'
    }
    if ($LASTEXITCODE -ne 0) { throw 'Adapter setup failed.' }
}
finally {
    $env:CTX_HOME = $previousCtxHome
    $env:CTX_BIN_DIR = $previousBinDir
}

$configFile = Join-Path $ConfigDir 'config.toml'
if (-not (Test-Path $configFile)) {
    @'
# ctx configuration
# docker_default = "desktop-linux"
# podman_default = "podman-machine-default"
# nerdctl_default = "default"
'@ | Set-Content -Encoding UTF8 $configFile
}

$completionPath = Join-Path $ConfigDir 'ctx-completion.ps1'
& $ctxTarget completion powershell | Set-Content -Encoding UTF8 $completionPath
if ($LASTEXITCODE -ne 0) { throw 'Failed to generate PowerShell completion' }

Write-Host "Installed ctx.exe and adapter catalog in $BinDir"
Write-Host "Config: $configFile"
Write-Host "PowerShell completion: add `. '$completionPath' to your PowerShell profile"
if (($env:PATH -split ';') -notcontains $BinDir) {
    Write-Host "Add this directory before Docker, Podman, and nerdctl on PATH: $BinDir"
}
if ($downloadRoot) { Remove-Item -Recurse -Force -ErrorAction SilentlyContinue $downloadRoot }
