[CmdletBinding(PositionalBinding = $false)]
param(
    [Parameter(Position = 0)][string]$Operation,
    [Parameter(Position = 1)][string]$Selection,
    [Parameter(Position = 2, ValueFromRemainingArguments = $true)][string[]]$Arguments
)

switch ($Operation) {
    'list' { exit 0 }
    'doctor' { 'skip OrbStack is available only on macOS'; exit 0 }
    'validate' { [Console]::Error.WriteLine('orbstack: OrbStack is unavailable on Windows'); exit 1 }
    'run' { [Console]::Error.WriteLine('orbstack: OrbStack is unavailable on Windows'); exit 1 }
    default { exit 2 }
}
