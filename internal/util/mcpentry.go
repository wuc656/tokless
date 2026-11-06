package util

import (
	"path/filepath"
	"strings"
)

// EqualPaths compares two filesystem paths tolerating case differences on Windows
// and slash variations.
func EqualPaths(a, b string) bool {
	if a == b {
		return true
	}
	if a == "" || b == "" {
		return false
	}
	cleanA := filepath.Clean(a)
	cleanB := filepath.Clean(b)
	if cleanA == cleanB {
		return true
	}
	if IsWin {
		if strings.EqualFold(cleanA, cleanB) {
			return true
		}
		baseA := strings.TrimSuffix(strings.ToLower(cleanA), filepath.Ext(cleanA))
		baseB := strings.TrimSuffix(strings.ToLower(cleanB), filepath.Ext(cleanB))
		if baseA == baseB {
			return true
		}
	}
	return false
}

// IsToklessCommand reports whether a command name/path points to tokless or toklessw.
func IsToklessCommand(cmd string) bool {
	base := strings.ToLower(filepath.Base(cmd))
	base = strings.TrimSuffix(base, ".exe")
	return base == "tokless" || base == "toklessw"
}

// CommandMatches reports whether have and want refer to the same binary or command.
func CommandMatches(have, want string) bool {
	if have == want {
		return true
	}
	if EqualPaths(have, want) {
		return true
	}
	if IsToklessCommand(have) && IsToklessCommand(want) {
		return true
	}
	baseHave := filepath.Base(have)
	baseWant := filepath.Base(want)
	if IsWin {
		baseHave = strings.ToLower(strings.TrimSuffix(baseHave, filepath.Ext(baseHave)))
		baseWant = strings.ToLower(strings.TrimSuffix(baseWant, filepath.Ext(baseWant)))
	}
	return baseHave == baseWant
}

// ArgsEqual checks if a (which can be []any or []string) equals b ([]string) with
// tolerance for case insensitivity and path format on Windows.
func ArgsEqual(a any, b []string) bool {
	var arr []string
	switch v := a.(type) {
	case []string:
		arr = v
	case []any:
		arr = make([]string, len(v))
		for i, x := range v {
			if s, ok := x.(string); ok {
				arr[i] = s
			} else {
				return false
			}
		}
	default:
		if a == nil && len(b) == 0 {
			return true
		}
		return false
	}
	if len(arr) != len(b) {
		return false
	}
	for i := range arr {
		if arr[i] == b[i] {
			continue
		}
		if IsWin && strings.EqualFold(arr[i], b[i]) {
			continue
		}
		if EqualPaths(arr[i], b[i]) {
			continue
		}
		return false
	}
	return true
}

// ExtractMcpCommandAndArgs extracts command and args from an arbitrary MCP entry representation.
func ExtractMcpCommandAndArgs(v any) (command string, args []string, ok bool) {
	if v == nil {
		return "", nil, false
	}
	switch m := v.(type) {
	case *OrderedMap:
		if cmdVal, has := m.Get("command"); has {
			if s, ok := cmdVal.(string); ok {
				command = s
			} else if slice, ok := cmdVal.([]any); ok && len(slice) > 0 {
				if first, ok := slice[0].(string); ok {
					command = first
				}
				for _, a := range slice[1:] {
					if as, ok := a.(string); ok {
						args = append(args, as)
					}
				}
			}
		}
		if argsVal, has := m.Get("args"); has {
			if slice, ok := argsVal.([]any); ok {
				for _, a := range slice {
					if as, ok := a.(string); ok {
						args = append(args, as)
					}
				}
			} else if sSlice, ok := argsVal.([]string); ok {
				args = append(args, sSlice...)
			}
		}
		return command, args, command != "" || len(args) > 0
	case map[string]any:
		if cmdVal, has := m["command"]; has {
			if s, ok := cmdVal.(string); ok {
				command = s
			} else if slice, ok := cmdVal.([]any); ok && len(slice) > 0 {
				if first, ok := slice[0].(string); ok {
					command = first
				}
				for _, a := range slice[1:] {
					if as, ok := a.(string); ok {
						args = append(args, as)
					}
				}
			}
		}
		if argsVal, has := m["args"]; has {
			if slice, ok := argsVal.([]any); ok {
				for _, a := range slice {
					if as, ok := a.(string); ok {
						args = append(args, as)
					}
				}
			} else if sSlice, ok := argsVal.([]string); ok {
				args = append(args, sSlice...)
			}
		}
		return command, args, command != "" || len(args) > 0
	case []string:
		if len(m) > 0 {
			return m[0], m[1:], true
		}
	case []any:
		if len(m) > 0 {
			if s, ok := m[0].(string); ok {
				var rest []string
				for _, x := range m[1:] {
					if xs, ok := x.(string); ok {
						rest = append(rest, xs)
					}
				}
				return s, rest, true
			}
		}
	case string:
		return m, nil, true
	}
	return "", nil, false
}

// IsToklessOrToolMcpEntry identifies whether an entry is managed by Tokless or belongs to toolID.
func IsToklessOrToolMcpEntry(toolID string, v any) bool {
	if v == nil {
		return false
	}
	cmd, args, ok := ExtractMcpCommandAndArgs(v)
	if !ok {
		return false
	}
	cmdLower := strings.ToLower(cmd)
	cmdBase := strings.ToLower(filepath.Base(cmd))
	cmdBase = strings.TrimSuffix(cmdBase, ".exe")
	cmdBase = strings.TrimSuffix(cmdBase, ".cmd")
	cmdBase = strings.TrimSuffix(cmdBase, ".bat")

	hasRunMcp := false
	for _, a := range args {
		if strings.ToLower(a) == "run-mcp" {
			hasRunMcp = true
			break
		}
	}

	// 1. Tokless command with run-mcp specifically targeting this toolID
	if IsToklessCommand(cmd) {
		if !hasRunMcp {
			return false
		}
		for i, a := range args {
			aBase := strings.ToLower(filepath.Base(a))
			aBase = strings.TrimSuffix(aBase, ".exe")
			aBase = strings.TrimSuffix(aBase, ".cmd")
			aBase = strings.TrimSuffix(aBase, ".bat")
			if aBase == toolID || (toolID == "context-mode" && (aBase == "context_mode" || a == "context-mode")) ||
				(toolID == "codegraph" && strings.Contains(aBase, "codegraph")) {
				return true
			}
			if a == "--tool" && i+1 < len(args) && strings.EqualFold(args[i+1], toolID) {
				return true
			}
		}
		return false
	}

	// 2. Direct tool invocation
	if cmdBase == toolID || (toolID == "context-mode" && cmdBase == "context_mode") {
		return true
	}
	if toolID == "codegraph" && (cmdBase == "codegraph" || strings.Contains(cmdLower, "@colbymchenry/codegraph")) {
		return true
	}

	// 3. Command wrapper (e.g. cmd.exe /c path\to\tool.cmd or npx --no-install tool)
	if (cmdBase == "cmd" || cmdBase == "npx") && len(args) > 0 {
		for _, a := range args {
			aBase := strings.ToLower(filepath.Base(a))
			aBase = strings.TrimSuffix(aBase, ".exe")
			aBase = strings.TrimSuffix(aBase, ".cmd")
			aBase = strings.TrimSuffix(aBase, ".bat")
			if aBase == toolID || (toolID == "context-mode" && aBase == "context_mode") ||
				(toolID == "codegraph" && strings.Contains(aBase, "codegraph")) {
				return true
			}
		}
	}

	return false
}

// McpEntryMatchesTool checks whether actual configured MCP entry matches expected command
// and arguments or is an equivalent Tokless-managed execution of the tool.
func McpEntryMatchesTool(toolID, expectedCmd string, expectedArgs []string, actual any) bool {
	if actual == nil {
		return false
	}
	cmd, args, ok := ExtractMcpCommandAndArgs(actual)
	if !ok {
		return false
	}
	// Direct command + args match with path normalization
	if CommandMatches(cmd, expectedCmd) && ArgsEqual(args, expectedArgs) {
		return true
	}
	// If both are Tokless bounded runs targeting toolID
	if IsToklessCommand(cmd) && IsToklessCommand(expectedCmd) {
		if IsToklessOrToolMcpEntry(toolID, actual) {
			if toolID == "codegraph" {
				hasServe := false
				hasMcp := false
				for _, a := range args {
					if a == "serve" {
						hasServe = true
					}
					if a == "--mcp" {
						hasMcp = true
					}
				}
				return hasServe && hasMcp
			}
			if toolID == "context-mode" {
				for _, a := range args {
					if a == "--context-mode" {
						return true
					}
				}
			}
		}
	}
	return false
}
