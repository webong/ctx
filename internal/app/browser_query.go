package app

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/webong/ctx/browser"
)

type browserQueryList []string

func (values *browserQueryList) String() string { return fmt.Sprint([]string(*values)) }
func (values *browserQueryList) Set(value string) error {
	if value == "" {
		return errors.New("query option cannot be empty")
	}
	*values = append(*values, value)
	return nil
}

func shareBrowserCookieQuery(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("share:browser cookie query", flag.ContinueOnError)
	flags.SetOutput(stderr)
	var sites, sources, browsers, names browserQueryList
	flags.Var(&sites, "site", "site URL (repeatable)")
	flags.Var(&sources, "from", "browser:profile source (repeatable, ordered)")
	flags.Var(&browsers, "browser", "browser adapter to discover (repeatable, ordered)")
	flags.Var(&names, "name", "cookie name (repeatable)")
	mode := flags.String("mode", string(browser.ModeMerge), "merge or first")
	toFile := flags.String("to-file", "", "new protected JSON result file")
	toStdout := flags.Bool("stdout", false, "write JSON result to a pipe")
	inlineFile := flags.String("inline-file", "", "inline cookie JSON file")
	inlineStdin := flags.Bool("inline-stdin", false, "read inline cookie JSON from a pipe")
	inlineOnly := flags.Bool("inline-only", false, "query inline cookies without browser adapters")
	includeExpired := flags.Bool("include-expired", false, "include expired cookies")
	allHosts := flags.Bool("all-hosts", false, "allow queries without a site URL")
	if err := flags.Parse(args); err != nil || len(flags.Args()) != 0 {
		return 2
	}
	if (*toFile == "") == !*toStdout || (*inlineFile != "" && *inlineStdin) {
		fmt.Fprintln(stderr, "ctx: query needs --to-file or --stdout and at most one inline input")
		return 2
	}
	if *toStdout {
		if err := requireOutputPipe(stdout); err != nil {
			return reportErrorCode(stderr, err, 2)
		}
	} else if err := requireNewOutputFile(*toFile); err != nil {
		return reportError(stderr, err)
	}
	options := browser.Options{
		Sources: append([]string(nil), sources...), Browsers: append([]string(nil), browsers...),
		Names: append([]string(nil), names...), Mode: browser.Mode(*mode),
		InlineOnly: *inlineOnly, IncludeExpired: *includeExpired, AllowAllHosts: *allHosts,
		Timeout: 2 * time.Minute,
	}
	if len(sites) > 0 {
		options.URL = sites[0]
		options.Origins = append([]string(nil), sites[1:]...)
	}
	if *inlineFile != "" {
		options.Inline.File = *inlineFile
	}
	if *inlineStdin {
		file, err := os.Stdin.Stat()
		if err != nil {
			return reportErrorCode(stderr, err, 2)
		}
		if file.Mode()&os.ModeNamedPipe == 0 {
			return reportErrorCode(stderr, errors.New("--inline-stdin requires a pipe"), 2)
		}
		data, err := io.ReadAll(io.LimitReader(os.Stdin, (8<<20)+1))
		if err != nil || len(data) > 8<<20 {
			return reportErrorCode(stderr, errors.New("inline cookie input exceeds 8 MiB or cannot be read"), 2)
		}
		options.Inline.JSON = data
	}
	result, err := browser.Get(context.Background(), options)
	if err != nil {
		return reportErrorCode(stderr, err, 2)
	}
	for _, warning := range result.Warnings {
		fmt.Fprintf(stderr, "ctx: warning: %s\n", warning)
	}
	if *toStdout {
		if err := json.NewEncoder(stdout).Encode(result); err != nil {
			return reportError(stderr, err)
		}
		return 0
	}
	if err := writePrivateJSON(*toFile, result); err != nil {
		return reportError(stderr, err)
	}
	fmt.Fprintf(stdout, "wrote %d cookies to %s (mode 0600)\n", len(result.Cookies), *toFile)
	return 0
}
