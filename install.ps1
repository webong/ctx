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
$bundleRoot = $null
$downloadRoot = $null

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
foreach ($engine in @('docker', 'podman', 'nerdctl')) {
    $existingShim = Join-Path $BinDir "$engine.cmd"
    if (Test-Path $existingShim) {
        if (-not (Select-String -Quiet -Path $existingShim -Pattern 'ctx native Windows shim')) {
            throw "$existingShim exists; choose another BinDir"
        }
    }
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

foreach ($engine in @('docker', 'podman', 'nerdctl')) {
    $shimTarget = Join-Path $BinDir "$engine.cmd"
    if ($bundleRoot) {
        Copy-Item (Join-Path $bundleRoot "bin\$engine.cmd") $shimTarget
    }
    else {
        $shim = "@echo off`r`nrem ctx native Windows shim for $engine`r`n`"%~dp0ctx.exe`" run $engine %*`r`nexit /b %ERRORLEVEL%`r`n"
        Set-Content -Encoding ASCII -Path $shimTarget -Value $shim -NoNewline
    }
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
    $adapterSource = if ($bundleRoot) { Join-Path $bundleRoot "adapters\$adapter" } else { Join-Path $repositoryRoot "adapters\$adapter" }
    Copy-Item -Recurse -Path $adapterSource -Destination $target
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

$completionPath = Join-Path $ConfigDir 'ctx-completion.ps1'
& $ctxTarget completion powershell | Set-Content -Encoding UTF8 $completionPath
if ($LASTEXITCODE -ne 0) { throw 'Failed to generate PowerShell completion' }

Write-Host "Installed ctx.exe, container shims, and first-party adapters in $BinDir"
Write-Host "Config: $configFile"
Write-Host "PowerShell completion: add `. '$completionPath' to your PowerShell profile"
if (($env:PATH -split ';') -notcontains $BinDir) {
    Write-Host "Add this directory before Docker, Podman, and nerdctl on PATH: $BinDir"
}
if ($downloadRoot) { Remove-Item -Recurse -Force -ErrorAction SilentlyContinue $downloadRoot }
Write-Warning 'Native Windows support is a migration preview; validate it with your local CLI and credential setup before replacing an existing installation.'
