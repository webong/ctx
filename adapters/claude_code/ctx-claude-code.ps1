[CmdletBinding(PositionalBinding = $false)]
param(
    [Parameter(Position = 0)][string]$Operation,
    [Parameter(Position = 1, ValueFromRemainingArguments = $true)][string[]]$Arguments
)

$claude = $env:CTX_ADAPTER_REAL_COMMAND
if ($Arguments.Count -gt 0 -and $Arguments[0] -eq '--') { $Arguments = @($Arguments | Select-Object -Skip 1) }
function Invoke-Claude([string[]]$CommandArguments) {
    & $claude @CommandArguments
    exit $LASTEXITCODE
}

switch ($Operation) {
    'validate' { if (-not $claude) { [Console]::Error.WriteLine('claude: Claude Code CLI is not installed'); exit 127 } }
    'doctor' {
        if (-not $claude) { [Console]::Error.WriteLine('claude: Claude Code CLI is not installed'); exit 127 }
        Invoke-Claude @('--version')
    }
    'run' {
        if (-not $claude) { [Console]::Error.WriteLine('claude: Claude Code CLI is not installed'); exit 127 }
        Invoke-Claude $Arguments
    }
    'hook' {
        if ($Arguments.Count -eq 0) { [Console]::Error.WriteLine('claude: hook needs an event name'); exit 2 }
        if (-not $env:CTX_COMPUTER_HOOK_COMMAND) { [Console]::Error.WriteLine('claude: set CTX_COMPUTER_HOOK_COMMAND to a hook handler executable'); exit 127 }
        & $env:CTX_COMPUTER_HOOK_COMMAND $Arguments[0]
        exit $LASTEXITCODE
    }
    'plugin' {
        if (-not $claude) { [Console]::Error.WriteLine('claude: Claude Code CLI is not installed'); exit 127 }
        Invoke-Claude (@('plugin') + $Arguments)
    }
    default { exit 2 }
}
