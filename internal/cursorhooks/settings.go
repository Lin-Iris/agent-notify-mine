package cursorhooks

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"

	"github.com/hellolib/agent-notify/internal/common"
)

const hookCommandMarker = "handle-cursor-hook"

type eventMeta struct {
	EventKey   string
	SubCommand string
}

var managedEvents = []eventMeta{
	{EventKey: "beforeShellExecution", SubCommand: "permission_required"},
	{EventKey: "stop", SubCommand: "run_completed"},
	{EventKey: "postToolUseFailure", SubCommand: "run_failed"},
}

func BuildHookSettings(binaryPath string) map[string]any {
	binaryPath = common.ResolveBinaryPath(binaryPath)
	hooks := map[string]any{}

	for _, evt := range managedEvents {
		command := binaryPath + " " + hookCommandMarker + " " + evt.SubCommand
		entry := map[string]any{
			"command": command,
		}
		hooks[evt.EventKey] = []any{entry}
	}

	return map[string]any{
		"version": 1,
		"hooks":   hooks,
	}
}

func Install(path string, binaryPath string) error {
	settings, err := readSettings(path)
	if err != nil {
		return err
	}

	binaryPath = common.ResolveBinaryPath(binaryPath)
	hooks, _ := settings["hooks"].(map[string]any)
	if hooks == nil {
		hooks = map[string]any{}
	}

	for _, evt := range managedEvents {
		if eventHasManagedHook(hooks, evt.EventKey) {
			continue
		}
		command := binaryPath + " " + hookCommandMarker + " " + evt.SubCommand
		entry := map[string]any{
			"command": command,
		}
		entries := toAnySlice(hooks[evt.EventKey])
		entries = append(entries, entry)
		hooks[evt.EventKey] = entries
	}
	settings["hooks"] = hooks
	if _, ok := settings["version"]; !ok {
		settings["version"] = float64(1)
	}

	return writeSettings(path, settings)
}

func IsInstalled(path string) (bool, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}

	settings := map[string]any{}
	if len(data) > 0 {
		if err := json.Unmarshal(data, &settings); err != nil {
			return false, err
		}
	}

	hooks, ok := settings["hooks"].(map[string]any)
	if !ok {
		return false, nil
	}

	for _, evt := range managedEvents {
		if eventHasManagedHook(hooks, evt.EventKey) {
			return true, nil
		}
	}
	return false, nil
}

func Uninstall(path string) error {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}

	settings := map[string]any{}
	if len(data) > 0 {
		if err := json.Unmarshal(data, &settings); err != nil {
			return err
		}
	}

	hooks, ok := settings["hooks"].(map[string]any)
	if !ok {
		return nil
	}

	for event, raw := range hooks {
		entries := toAnySlice(raw)
		cleaned := entries[:0]
		for _, entry := range entries {
			if !isManagedHook(entry) {
				cleaned = append(cleaned, entry)
			}
		}
		if len(cleaned) == 0 {
			delete(hooks, event)
		} else {
			hooks[event] = cleaned
		}
	}

	if len(hooks) == 0 {
		delete(settings, "hooks")
	} else {
		settings["hooks"] = hooks
	}

	return writeSettings(path, settings)
}

func readSettings(path string) (map[string]any, error) {
	settings := map[string]any{}
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return settings, nil
		}
		return nil, err
	}
	if len(data) == 0 {
		return settings, nil
	}
	if err := json.Unmarshal(data, &settings); err != nil {
		return nil, err
	}
	return settings, nil
}

func writeSettings(path string, settings map[string]any) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	out, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, out, 0o644)
}

func eventHasManagedHook(hooks map[string]any, event string) bool {
	for _, entry := range toAnySlice(hooks[event]) {
		if isManagedHook(entry) {
			return true
		}
	}
	return false
}

func isManagedHook(hook any) bool {
	m, ok := hook.(map[string]any)
	if !ok {
		return false
	}
	cmd, _ := m["command"].(string)
	return strings.Contains(cmd, hookCommandMarker)
}

func toAnySlice(v any) []any {
	switch s := v.(type) {
	case []any:
		return s
	case []map[string]any:
		out := make([]any, 0, len(s))
		for _, item := range s {
			out = append(out, item)
		}
		return out
	default:
		return nil
	}
}
