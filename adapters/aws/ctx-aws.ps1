[CmdletBinding(PositionalBinding = $false)]
param(
    [Parameter(Position = 0)][string]$Operation,
    [Parameter(Position = 1)][string]$Selection,
    [Parameter(Position = 2, ValueFromRemainingArguments = $true)][string[]]$Arguments
)

$aws = (Get-Command aws -CommandType Application -ErrorAction SilentlyContinue | Select-Object -First 1).Source
if (-not $aws) { [Console]::Error.WriteLine('aws: AWS CLI is not installed'); exit 127 }

function Get-Profiles { @(& $aws configure list-profiles) }
function Test-Profile([string]$Name) { return (Get-Profiles) -contains $Name }

switch ($Operation) {
    'list' { & $aws configure list-profiles; exit $LASTEXITCODE }
    'validate' {
        if (-not (Test-Profile $Selection)) { [Console]::Error.WriteLine("aws: unknown profile $Selection"); exit 1 }
    }
    'doctor' {
        if ($Selection -and -not (Test-Profile $Selection)) { [Console]::Error.WriteLine("aws: unknown profile $Selection"); exit 1 }
    }
    'run' {
        if ($Arguments.Count -gt 0 -and $Arguments[0] -eq '--') { $Arguments = @($Arguments | Select-Object -Skip 1) }
        $hasProfile = $env:AWS_PROFILE -or $env:AWS_DEFAULT_PROFILE
        foreach ($argument in $Arguments) { if ($argument -eq '--profile' -or $argument -like '--profile=*') { $hasProfile = $true } }
        if ($hasProfile) { & $aws @Arguments } else { & $aws --profile $Selection @Arguments }
        exit $LASTEXITCODE
    }
    default { exit 2 }
}
