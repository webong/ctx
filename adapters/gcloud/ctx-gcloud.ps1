[CmdletBinding(PositionalBinding = $false)]
param(
    [Parameter(Position = 0)][string]$Operation,
    [Parameter(Position = 1)][string]$Selection,
    [Parameter(Position = 2, ValueFromRemainingArguments = $true)][string[]]$Arguments
)

$gcloud = (Get-Command gcloud -CommandType Application -ErrorAction SilentlyContinue).Source
if (-not $gcloud) { [Console]::Error.WriteLine('gcloud: Google Cloud CLI is not installed'); exit 127 }

function Test-Configuration([string]$Name) {
    $result = @(& $gcloud config configurations list "--filter=name=$Name" '--format=value(name)')
    return $result -contains $Name
}

switch ($Operation) {
    'list' { & $gcloud config configurations list; exit $LASTEXITCODE }
    'validate' {
        if (-not (Test-Configuration $Selection)) { [Console]::Error.WriteLine("gcloud: unknown configuration $Selection"); exit 1 }
    }
    'doctor' {
        if ($Selection -and -not (Test-Configuration $Selection)) { [Console]::Error.WriteLine("gcloud: unknown configuration $Selection"); exit 1 }
    }
    'run' {
        if ($Arguments.Count -gt 0 -and $Arguments[0] -eq '--') { $Arguments = @($Arguments | Select-Object -Skip 1) }
        $hasConfiguration = [bool]$env:CLOUDSDK_ACTIVE_CONFIG_NAME
        foreach ($argument in $Arguments) { if ($argument -eq '--configuration' -or $argument -like '--configuration=*') { $hasConfiguration = $true } }
        if ($hasConfiguration) { & $gcloud @Arguments } else { & $gcloud --configuration $Selection @Arguments }
        exit $LASTEXITCODE
    }
    default { exit 2 }
}
