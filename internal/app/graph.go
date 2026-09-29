package app

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/webong/ctx/graph"
	"github.com/webong/ctx/internal/config"
	"github.com/webong/ctx/internal/systemgraph"
)

func graphCommand(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		args = []string{"status"}
	}
	system, err := systemgraph.Open(configHomePath())
	if err != nil {
		fmt.Fprintf(stderr, "ctx: open graph: %v\n", err)
		return 1
	}
	defer system.Close()
	if resolver, resolveErr := newResolver(); resolveErr == nil {
		profile, _, _ := resolver.ActiveProfile()
		if observeErr := system.ObserveShell(context.Background(), currentDirectory(), profile, observedSelections(resolver)); observeErr != nil {
			fmt.Fprintf(stderr, "ctx: warning: could not observe shell for graph query: %v\n", observeErr)
		}
	}
	ctx := context.Background()
	encoder := json.NewEncoder(stdout)
	encoder.SetIndent("", "  ")
	switch args[0] {
	case "status":
		if len(args) != 1 {
			fmt.Fprintln(stderr, "ctx: graph status takes no arguments")
			return 2
		}
		snapshot, err := system.Store.Snapshot(ctx, systemgraph.Namespace)
		if err != nil {
			fmt.Fprintf(stderr, "ctx: %v\n", err)
			return 1
		}
		fmt.Fprintf(stdout, "store: %s/graph.json\nnamespace: %s\nrevision: %d\ncursor: %d\nvertices: %d\nedges: %d\n", configHomePath(), snapshot.Namespace, snapshot.Revision, snapshot.Cursor, len(snapshot.Vertices), len(snapshot.Edges))
		return 0
	case "vertices":
		kind := ""
		if len(args) > 2 {
			fmt.Fprintln(stderr, "ctx: graph vertices accepts at most one kind")
			return 2
		}
		if len(args) == 2 {
			kind = args[1]
			if kind != "" && !strings.HasPrefix(kind, systemgraph.Namespace+"/") {
				kind = systemgraph.Namespace + "/" + kind
			}
		}
		values, err := system.Store.QueryVertices(ctx, graph.VertexQuery{Namespace: systemgraph.Namespace, Kind: kind, Limit: 1000})
		if err != nil {
			fmt.Fprintf(stderr, "ctx: %v\n", err)
			return 1
		}
		if err = encoder.Encode(values); err != nil {
			fmt.Fprintf(stderr, "ctx: %v\n", err)
			return 1
		}
		return 0
	case "edges":
		relationship := ""
		if len(args) > 2 {
			fmt.Fprintln(stderr, "ctx: graph edges accepts at most one relationship")
			return 2
		}
		if len(args) == 2 {
			relationship = args[1]
			if relationship != "" && !strings.HasPrefix(relationship, systemgraph.Namespace+"/") {
				relationship = systemgraph.Namespace + "/" + relationship
			}
		}
		values, err := system.Store.QueryEdges(ctx, graph.EdgeQuery{Namespace: systemgraph.Namespace, Type: relationship, Limit: 1000})
		if err != nil {
			fmt.Fprintf(stderr, "ctx: %v\n", err)
			return 1
		}
		if err = encoder.Encode(values); err != nil {
			fmt.Fprintf(stderr, "ctx: %v\n", err)
			return 1
		}
		return 0
	case "snapshot":
		if len(args) != 1 {
			fmt.Fprintln(stderr, "ctx: graph snapshot takes no arguments")
			return 2
		}
		snapshot, err := system.Store.Snapshot(ctx, systemgraph.Namespace)
		if err != nil {
			fmt.Fprintf(stderr, "ctx: %v\n", err)
			return 1
		}
		if err = encoder.Encode(snapshot); err != nil {
			fmt.Fprintf(stderr, "ctx: %v\n", err)
			return 1
		}
		return 0
	case "changes":
		if len(args) > 2 {
			fmt.Fprintln(stderr, "ctx: graph changes accepts at most one cursor")
			return 2
		}
		var cursor uint64
		if len(args) == 2 {
			cursor, err = strconv.ParseUint(args[1], 10, 64)
			if err != nil {
				fmt.Fprintln(stderr, "ctx: graph changes cursor must be an unsigned integer")
				return 2
			}
		}
		changes, err := system.Store.Changes(ctx, systemgraph.Namespace, cursor, 1000)
		if err != nil {
			fmt.Fprintf(stderr, "ctx: %v\n", err)
			return 1
		}
		if err = encoder.Encode(changes); err != nil {
			fmt.Fprintf(stderr, "ctx: %v\n", err)
			return 1
		}
		return 0
	default:
		fmt.Fprintf(stderr, "ctx: unknown graph command %s\n", args[0])
		fmt.Fprintln(stderr, "ctx: use graph status, vertices, edges, snapshot, or changes [cursor]")
		return 2
	}
}

func recordSystemContext(resolver *config.Resolver, stderr io.Writer) {
	system, err := systemgraph.Open(configHomePath())
	if err != nil {
		fmt.Fprintf(stderr, "ctx: warning: could not record system context: %v\n", err)
		return
	}
	defer system.Close()
	profile, _, _ := resolver.ActiveProfile()
	cwd := currentDirectory()
	selections := observedSelections(resolver)
	if err := system.ObserveShell(context.Background(), cwd, profile, selections); err != nil {
		fmt.Fprintf(stderr, "ctx: warning: could not record system context: %v\n", err)
	}
}

func observedSelections(resolver *config.Resolver) map[string]string {
	selections := map[string]string{}
	if installed, err := adapterStore().List(); err == nil {
		for _, candidate := range installed {
			if !candidate.IsSelectable() {
				continue
			}
			if resolved, resolveErr := resolver.Resolve(candidate.Manifest.SelectorKey); resolveErr == nil && resolved.Value != "" {
				selections[candidate.Manifest.SelectorKey] = resolved.Value
			}
		}
	}
	return selections
}

func recordWebContext(provider, profile string, args []string, stderr io.Writer) {
	system, err := systemgraph.Open(configHomePath())
	if err != nil {
		fmt.Fprintf(stderr, "ctx: warning: could not record web context: %v\n", err)
		return
	}
	defer system.Close()
	for _, arg := range args {
		scheme, host, ok := systemgraph.SafeOrigin(arg)
		if !ok {
			continue
		}
		if err := system.ObserveWeb(context.Background(), provider, profile, scheme, host); err != nil {
			fmt.Fprintf(stderr, "ctx: warning: could not record web context: %v\n", err)
			return
		}
	}
}
