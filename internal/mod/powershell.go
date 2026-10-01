package mod

import (
	"encoding/base64"
	"encoding/binary"
	"strings"
	"unicode/utf16"
)

// powershellArguments passes the adapter protocol as literal string values.
// Windows PowerShell -File interprets forwarded CLI flags (including --) as
// named script parameters, even with ValueFromRemainingArguments enabled.
func powershellArguments(path string, args []string) []string {
	literal := func(value string) string {
		return "'" + strings.ReplaceAll(value, "'", "''") + "'"
	}
	values := make([]string, len(args))
	for i, arg := range args {
		values[i] = literal(arg)
	}
	script := strings.Join([]string{
		"[Console]::OutputEncoding = [Text.UTF8Encoding]::new($false)",
		"$OutputEncoding = [Console]::OutputEncoding",
		"$ctxInvocationArguments = @(" + strings.Join(values, ",") + ")",
		"& " + literal(path) + " @ctxInvocationArguments",
		"if (-not $?) { if ($LASTEXITCODE) { exit $LASTEXITCODE }; exit 1 }",
		"exit $LASTEXITCODE",
	}, "; ")
	units := utf16.Encode([]rune(script))
	encoded := make([]byte, 2*len(units))
	for i, unit := range units {
		binary.LittleEndian.PutUint16(encoded[i*2:], unit)
	}
	return []string{"-NoLogo", "-NoProfile", "-EncodedCommand", base64.StdEncoding.EncodeToString(encoded)}
}
