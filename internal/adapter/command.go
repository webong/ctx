package adapter

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

type Invocation struct {
	Operation string
	Selection string
	Arguments []string
	Values    map[string]string
	Profile   string
	Project   string
	Command   string
}

func (a *Adapter) Command(invocation Invocation) (*exec.Cmd, error) {
	if !a.HasCapability(invocation.Operation) {
		return nil, fmt.Errorf("adapter %s does not support %s", a.Manifest.Name, invocation.Operation)
	}
	var args []string
	switch invocation.Operation {
	case "list":
		args = []string{"list"}
	case "validate", "doctor":
		args = []string{invocation.Operation, invocation.Selection}
	case "configure", "run", "open":
		args = append([]string{invocation.Operation, invocation.Selection, "--"}, invocation.Arguments...)
	default:
		return nil, fmt.Errorf("unsupported adapter operation %s", invocation.Operation)
	}
	command := adapterCommand(a.ExecutablePath(), args)
	command.Env = os.Environ()
	command.Env = setEnvironment(command.Env, "CTX_ADAPTER_API", APIVersion)
	command.Env = setEnvironment(command.Env, "CTX_ADAPTER_NAME", a.Manifest.Name)
	requestedCommand := invocation.Command
	if requestedCommand == "" {
		requestedCommand = a.Manifest.Name
	}
	command.Env = setEnvironment(command.Env, "CTX_ADAPTER_COMMAND", requestedCommand)
	command.Env = setEnvironment(command.Env, "CTX_PROJECT_DIR", invocation.Project)
	command.Env = setEnvironment(command.Env, "CTX_PROFILE", invocation.Profile)
	for key, value := range invocation.Values {
		environmentKey := "CTX_ADAPTER_VALUE_" + strings.ToUpper(key)
		command.Env = setEnvironment(command.Env, environmentKey, value)
	}
	return command, nil
}

func setEnvironment(values []string, key, value string) []string {
	prefix := strings.ToUpper(key) + "="
	for index, entry := range values {
		if strings.HasPrefix(strings.ToUpper(entry), prefix) {
			values[index] = key + "=" + value
			return values
		}
	}
	return append(values, key+"="+value)
}
