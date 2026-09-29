package app

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

	"github.com/webong/ctx/graph/system"
	"github.com/webong/ctx/internal/config"
	modpkg "github.com/webong/ctx/internal/mod"
)

// scanMachineInventory asks installed adapters for their declared capabilities
// and, where list has a line-oriented contract, configured contexts. The graph
// is an observation for discovery, never authority to execute an adapter.
func scanMachineInventory(resolver *config.Resolver, system *systemgraph.Graph) ([]systemgraph.AdapterObservation, error) {
	store := adapterStore()
	installed, err := store.List()
	if err != nil {
		return nil, err
	}
	observations := make([]systemgraph.AdapterObservation, len(installed))
	errors := make([]error, len(installed))
	var workers sync.WaitGroup
	limit := make(chan struct{}, 4)
	for index, candidate := range installed {
		workers.Add(1)
		limit <- struct{}{}
		go func(index int, candidate *modpkg.Adapter) {
			defer workers.Done()
			defer func() { <-limit }()
			observations[index], errors[index] = observeAdapterCandidate(resolver, store, candidate)
		}(index, candidate)
	}
	workers.Wait()
	for _, err := range errors {
		if err != nil {
			return nil, err
		}
	}
	if system == nil {
		return observations, nil
	}
	registry, err := readVirtualizerRegistry()
	if err != nil {
		return observations, err
	}
	aliases := make([]systemgraph.AliasObservation, 0, len(registry.Instances))
	for _, instance := range registry.Instances {
		metadata := map[string]string{}
		if instance.Virtualizer != "" {
			metadata["virtualizer"] = instance.Virtualizer
		}
		if instance.Machine != "" {
			metadata["machine"] = instance.Machine
		}
		aliases = append(aliases, systemgraph.AliasObservation{
			Space: "virtualizer", Name: instance.Name, Adapter: instance.Provider,
			Selection: instance.Selection, Metadata: metadata,
		})
	}
	return observations, system.ObserveInventory(context.Background(), observations, aliases)
}

func observeAdapterCandidate(resolver *config.Resolver, store *modpkg.Store, candidate *modpkg.Adapter) (systemgraph.AdapterObservation, error) {
	trusted, err := store.IsTrusted(candidate)
	if err != nil {
		return systemgraph.AdapterObservation{}, err
	}
	observation := systemgraph.AdapterObservation{
		Name: candidate.Manifest.Name, Runtime: candidate.Manifest.Runtime,
		Selector:         candidate.Manifest.SelectorKey,
		Surfaces:         append([]string(nil), candidate.Manifest.Surfaces...),
		Capabilities:     append([]string(nil), candidate.Manifest.Capabilities...),
		Trusted:          trusted,
		ListSupported:    candidate.HasCapability("list"),
		ObserveSupported: candidate.HasCapability("observe"),
		DiscoveryStatus:  "not-supported",
	}
	if !trusted {
		observation.DiscoveryStatus = "untrusted"
	}
	if trusted && candidate.HasCapability("observe") {
		var output boundedObservationBuffer
		var adapterError boundedObservationBuffer
		if code := invokeAdapter(resolver, candidate, "observe", "", nil, "", &output, &adapterError); code == 0 {
			contexts, resources, relations, parseErr := parseAdapterObservation(output.Bytes(), candidate.Manifest.Capabilities)
			if parseErr != nil {
				observation.DiscoveryStatus = "invalid-response"
			} else {
				observation.Listed = true
				observation.DiscoveryStatus = "ok"
				observation.Contexts = contexts
				observation.Resources = resources
				observation.Relations = relations
			}
		} else {
			observation.DiscoveryStatus = fmt.Sprintf("exit:%d", code)
		}
	} else if trusted && candidate.HasCapability("list") {
		var output boundedObservationBuffer
		var adapterError boundedObservationBuffer
		if code := invokeAdapter(resolver, candidate, "list", "", nil, "", &output, &adapterError); code == 0 {
			observation.Listed = true
			observation.DiscoveryStatus = "ok"
			if candidate.IsRuntime("browser") || candidate.IsRuntime("virtualizer") {
				seen := map[string]bool{}
				for _, line := range strings.Split(output.String(), "\n") {
					selection := strings.TrimSuffix(line, "\r")
					if candidate.IsRuntime("browser") {
						selection = strings.TrimPrefix(selection, candidate.Manifest.Name+":")
					}
					if selection != "" && !seen[selection] {
						observation.Contexts = append(observation.Contexts, systemgraph.ContextObservation{Selection: selection})
						seen[selection] = true
					}
				}
			}
		} else {
			observation.DiscoveryStatus = fmt.Sprintf("exit:%d", code)
		}
	}
	return observation, nil
}

func scanMachineInventoryInGraph(resolver *config.Resolver) ([]systemgraph.AdapterObservation, error) {
	system, err := systemgraph.Open(configHomePath())
	if err != nil {
		return nil, err
	}
	defer system.Close()
	return scanMachineInventory(resolver, system)
}

func resolvedMachineInventory(resolver *config.Resolver) (systemgraph.Inventory, error) {
	system, err := systemgraph.Open(configHomePath())
	if err != nil {
		return systemgraph.Inventory{}, err
	}
	defer system.Close()
	if _, err := scanMachineInventory(resolver, system); err != nil {
		return systemgraph.Inventory{}, err
	}
	return system.ResolveInventory(context.Background(), "")
}

func freshMachineInventory(resolver *config.Resolver) (systemgraph.Inventory, error) {
	system, err := systemgraph.Open(configHomePath())
	if err != nil {
		return systemgraph.Inventory{}, err
	}
	defer system.Close()
	inventory, err := system.ResolveInventory(context.Background(), "")
	if err != nil {
		return systemgraph.Inventory{}, err
	}
	if inventory.FreshWithin(5 * time.Second) {
		return inventory, nil
	}
	if _, err := scanMachineInventory(resolver, system); err != nil {
		return systemgraph.Inventory{}, err
	}
	return system.ResolveInventory(context.Background(), "")
}

func printDiscoveredVirtualizers(inventory systemgraph.Inventory, stdout io.Writer) int {
	count := 0
	for _, candidate := range inventory.Contexts {
		if candidate.Runtime != "virtualizer" || candidate.Selection == "" {
			continue
		}
		fmt.Fprintf(stdout, "%s:%s\n", candidate.Adapter, candidate.Selection)
		count++
	}
	return count
}

type boundedObservationBuffer struct{ bytes.Buffer }

func (buffer *boundedObservationBuffer) Write(data []byte) (int, error) {
	if buffer.Len()+len(data) > maxAdapterObservationBytes {
		return 0, fmt.Errorf("adapter observation exceeds 1 MiB")
	}
	return buffer.Buffer.Write(data)
}
