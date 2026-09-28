package engine

import "strings"

type Environment interface {
	Get(string) string
}

type MapEnvironment map[string]string

func (e MapEnvironment) Get(key string) string { return e[key] }

func Arguments(name, selection string, args []string, env Environment) []string {
	result := append([]string(nil), args...)
	if selection == "" || explicit(name, args, env) {
		return result
	}
	switch name {
	case "docker":
		return append([]string{"--context", selection}, result...)
	case "podman":
		return append([]string{"--connection", selection}, result...)
	case "nerdctl":
		return append([]string{"--namespace", selection}, result...)
	default:
		return result
	}
}

func explicit(name string, args []string, env Environment) bool {
	switch name {
	case "docker":
		if env.Get("DOCKER_CONTEXT") != "" || env.Get("DOCKER_HOST") != "" || first(args) == "context" {
			return true
		}
		return hasFlag(args, "--context", "--host", "-H")
	case "podman":
		if env.Get("CONTAINER_CONNECTION") != "" || env.Get("CONTAINER_HOST") != "" {
			return true
		}
		if first(args) == "machine" || (len(args) > 1 && args[0] == "system" && args[1] == "connection") {
			return true
		}
		return hasFlag(args, "--connection", "--url", "--remote", "-c", "-r")
	case "nerdctl":
		if env.Get("CONTAINERD_NAMESPACE") != "" || env.Get("CONTAINERD_ADDRESS") != "" || first(args) == "namespace" {
			return true
		}
		return hasFlag(args, "--namespace", "--address", "-n", "-a")
	}
	return false
}

func first(args []string) string {
	if len(args) == 0 {
		return ""
	}
	return args[0]
}

func hasFlag(args []string, names ...string) bool {
	for _, arg := range args {
		for _, name := range names {
			if arg == name || strings.HasPrefix(arg, name+"=") || (len(name) == 2 && strings.HasPrefix(arg, name) && len(arg) > 2) {
				return true
			}
		}
	}
	return false
}
