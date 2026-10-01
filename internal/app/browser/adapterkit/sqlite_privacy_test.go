package adapterkit

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestSQLiteKeepsCookieValuesOffProcessArgumentsAndErrors(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("fixture uses a POSIX shell")
	}
	root := t.TempDir()
	arguments, input := filepath.Join(root, "arguments"), filepath.Join(root, "input")
	t.Setenv("CTX_TEST_SQLITE_ARGUMENTS", arguments)
	t.Setenv("CTX_TEST_SQLITE_INPUT", input)
	t.Setenv("PATH", root+string(os.PathListSeparator)+os.Getenv("PATH"))
	script := `#!/bin/sh
printf '%s\n' "$@" > "$CTX_TEST_SQLITE_ARGUMENTS"
cat > "$CTX_TEST_SQLITE_INPUT"
printf '%s' 'parse error: fixture-secret' >&2
exit 1
`
	if err := os.WriteFile(filepath.Join(root, "sqlite3"), []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	_, err := RunSQLite(filepath.Join(root, "cookies.sqlite"), false, "INSERT INTO cookies(value) VALUES('fixture-secret')")
	if err == nil || strings.Contains(err.Error(), "fixture-secret") {
		t.Fatal("write diagnostics exposed a cookie value")
	}
	argv, err := os.ReadFile(arguments)
	if err != nil || strings.Contains(string(argv), "fixture-secret") {
		t.Fatal("SQL appeared in process arguments")
	}
	data, err := os.ReadFile(input)
	if err != nil || !strings.Contains(string(data), "fixture-secret") {
		t.Fatal("SQL was not sent through stdin")
	}
}
