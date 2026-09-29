[CmdletBinding(PositionalBinding = $false)]
param(
    [Parameter(Position = 0)][string]$Operation,
    [Parameter(Position = 1, ValueFromRemainingArguments = $true)][string[]]$Arguments
)

$codex = $env:CTX_ADAPTER_REAL_COMMAND
if ($Arguments.Count -gt 0 -and $Arguments[0] -eq '--') { $Arguments = @($Arguments | Select-Object -Skip 1) }
function Invoke-Codex([string[]]$CommandArguments) {
    & $codex @CommandArguments
    exit $LASTEXITCODE
}

switch ($Operation) {
    'validate' { if (-not $codex) { [Console]::Error.WriteLine('codex: Codex CLI is not installed'); exit 127 } }
    'doctor' {
        if (-not $codex) { [Console]::Error.WriteLine('codex: Codex CLI is not installed'); exit 127 }
        Invoke-Codex @('--version')
    }
    'run' {
        if (-not $codex) { [Console]::Error.WriteLine('codex: Codex CLI is not installed'); exit 127 }
        Invoke-Codex $Arguments
    }
    'hook' {
        if ($Arguments.Count -eq 0) { [Console]::Error.WriteLine('codex: hook needs an event name'); exit 2 }
        if (-not $env:CTX_COMPUTER_HOOK_COMMAND) { [Console]::Error.WriteLine('codex: set CTX_COMPUTER_HOOK_COMMAND to a hook handler executable'); exit 127 }
        & $env:CTX_COMPUTER_HOOK_COMMAND $Arguments[0]
        exit $LASTEXITCODE
    }
    'plugin' {
        if (-not $codex) { [Console]::Error.WriteLine('codex: Codex CLI is not installed'); exit 127 }
        Invoke-Codex (@('plugin') + $Arguments)
    }
    default { exit 2 }
}
