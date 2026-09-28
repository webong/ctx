package app

import (
	"fmt"
	"io"
)

func completion(args []string, stdout, stderr io.Writer) int {
	if len(args) != 1 || args[0] != "powershell" {
		fmt.Fprintln(stderr, "ctx: completion currently supports powershell")
		return 2
	}
	fmt.Fprint(stdout, powershellCompletion)
	return 0
}

const powershellCompletion = `# ctx PowerShell completion
Register-ArgumentCompleter -Native -CommandName ctx -ScriptBlock {
    param($wordToComplete, $commandAst, $cursorPosition)

    $words = @($commandAst.CommandElements | ForEach-Object { $_.Extent.Text })
    $commands = @('status', 'resolve', 'explain', 'env', 'set', 'clear', 'profile', 'adapter', 'setup', 'ls', 'open', 'doctor', 'build', 'image', 'volume', 'run', 'shell', 'real', 'completion', 'version')

    function Emit-CtxCompletion([string[]]$values) {
        $values | Where-Object { $_ -and $_ -like "$wordToComplete*" } | Sort-Object -Unique | ForEach-Object {
            [System.Management.Automation.CompletionResult]::new($_, $_, 'ParameterValue', $_)
        }
    }
    function Get-CtxSelectors {
        $values = @('browser')
        $values += @(& ctx adapter ls 2>$null | ForEach-Object { ($_ -split '\s+')[0] })
        return $values
    }
    function Get-CtxContainerProviders {
        @(& ctx adapter ls container 2>$null | ForEach-Object { ($_ -split '\s+')[0] })
    }
    function Get-CtxAvailableAdapters {
        @(& ctx adapter available 2>$null | Where-Object { $_ -match '\savailable\s' } | ForEach-Object { ($_ -split '\s+')[0] })
    }
    function Get-CtxProfiles { @(& ctx profile ls 2>$null) }

    if ($words.Count -le 1) { Emit-CtxCompletion $commands; return }
    $subcommand = $words[1]
    if ($words.Count -eq 2 -and $commands -notcontains $subcommand) { Emit-CtxCompletion $commands; return }

    switch ($subcommand) {
        { $_ -in @('status', 'ls', 'set') } {
            if ($words.Count -le 2) { Emit-CtxCompletion (Get-CtxSelectors); return }
            if ($subcommand -eq 'set' -and $words.Count -le 3) {
                Emit-CtxCompletion @(& ctx ls $words[2] 2>$null); return
            }
        }
        'clear' { if ($words.Count -le 2) { Emit-CtxCompletion ((Get-CtxSelectors) + 'profile'); return } }
        'completion' { if ($words.Count -le 2) { Emit-CtxCompletion @('powershell'); return } }
        'image' { if ($words.Count -le 2) { Emit-CtxCompletion @('sync', 'copy'); return } }
        'volume' { if ($words.Count -le 2) { Emit-CtxCompletion @('export', 'import', 'copy'); return } }
        'build' { if ($words.Count -le 2) { Emit-CtxCompletion ((Get-CtxContainerProviders) + '--cache-ref'); return } }
        'profile' {
            $operations = @('ls', 'show', 'use', 'set', 'unset', 'env', 'env-unset', 'clear')
            if ($words.Count -le 2) { Emit-CtxCompletion $operations; return }
            if ($words[2] -in @('show', 'use', 'set', 'unset', 'env', 'env-unset') -and $words.Count -le 3) {
                Emit-CtxCompletion (Get-CtxProfiles); return
            }
        }
        'adapter' {
            $operations = @('ls', 'available', 'add', 'refresh', 'inspect', 'install', 'trust', 'test', 'doctor', 'remove')
            if ($words.Count -le 2) { Emit-CtxCompletion $operations; return }
            if ($words[2] -in @('inspect', 'trust', 'doctor', 'remove') -and $words.Count -le 3) {
                Emit-CtxCompletion @(& ctx adapter ls 2>$null | ForEach-Object { ($_ -split '\s+')[0] }); return
            }
            if ($words[2] -eq 'add') { Emit-CtxCompletion (Get-CtxAvailableAdapters); return }
        }
        'setup' { if ($words.Count -le 2) { Emit-CtxCompletion @('adapters', '--all', '--minimal', '--adapters'); return } }
        'shell' { if ($words.Count -le 2) { Emit-CtxCompletion @('--shell', '--'); return } }
    }
}
`
