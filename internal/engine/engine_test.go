package engine

import (
	"reflect"
	"testing"
)

func TestArguments(t *testing.T) {
	tests := []struct {
		name      string
		selection string
		args      []string
		env       MapEnvironment
		want      []string
	}{
		{"docker", "orbstack", []string{"ps"}, nil, []string{"--context", "orbstack", "ps"}},
		{"docker", "orbstack", []string{"--context", "remote", "ps"}, nil, []string{"--context", "remote", "ps"}},
		{"podman", "machine", []string{"images"}, nil, []string{"--connection", "machine", "images"}},
		{"podman", "machine", []string{"machine", "start"}, nil, []string{"machine", "start"}},
		{"nerdctl", "k8s.io", []string{"ps"}, nil, []string{"--namespace", "k8s.io", "ps"}},
		{"nerdctl", "k8s.io", []string{"ps"}, MapEnvironment{"CONTAINERD_NAMESPACE": "default"}, []string{"ps"}},
	}
	for _, test := range tests {
		if got := Arguments(test.name, test.selection, test.args, test.env); !reflect.DeepEqual(got, test.want) {
			t.Fatalf("%s: got %#v, want %#v", test.name, got, test.want)
		}
	}
}
