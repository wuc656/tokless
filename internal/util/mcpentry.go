package util

import (
	"path/filepath"
	"strconv"
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

// ToAnySlice converts []string into []any.
func ToAnySlice(ss []string) []any {
	out := make([]any, len(ss))
	for i, s := range ss {
		out[i] = s
	}
	return out
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

// IsToklessManagedMcpEntry reports whether an MCP config entry was written by Tokless
// (invoking tokless/toklessw with run-mcp or a bare tokless executable stub).
func IsToklessManagedMcpEntry(v any) bool {
	cmd, args, ok := ExtractMcpCommandAndArgs(v)
	if !ok || !IsToklessCommand(cmd) {
		return false
	}
	if len(args) == 0 {
		return true
	}
	for _, a := range args {
		if strings.EqualFold(a, "run-mcp") {
			return true
		}
	}
	return false
}

func parseTomlStringArrayInBlock(blk, key string) ([]any, bool, bool) {
	lines := strings.Split(blk, "\n")
	startIdx := -1
	var afterEq string
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, key) {
			continue
		}
		if eq := strings.IndexByte(trimmed, '='); eq > 0 && strings.TrimSpace(trimmed[:eq]) == key {
			startIdx = i
			afterEq = strings.TrimSpace(stripTomlComment(trimmed[eq+1:]))
			break
		}
	}
	if startIdx < 0 {
		return nil, false, true
	}
	if !strings.HasPrefix(afterEq, "[") {
		return nil, true, false
	}

	var buf strings.Builder
	depth := 0
	inStr, inLit, esc := false, false, false
	closed := false
	for i := startIdx; i < len(lines) && !closed; i++ {
		chunk := afterEq
		if i > startIdx {
			chunk = strings.TrimSpace(stripTomlComment(lines[i]))
			if buf.Len() > 0 && chunk != "" {
				buf.WriteByte('\n')
			}
		}
		for j := 0; j < len(chunk); j++ {
			c := chunk[j]
			buf.WriteByte(c)
			if inStr {
				if esc {
					esc = false
				} else if c == '\\' {
					esc = true
				} else if c == '"' {
					inStr = false
				}
				continue
			}
			if inLit {
				if c == '\'' {
					inLit = false
				}
				continue
			}
			switch c {
			case '"':
				inStr = true
			case '\'':
				inLit = true
			case '[':
				depth++
			case ']':
				depth--
				if depth == 0 {
					if strings.TrimSpace(chunk[j+1:]) != "" {
						return nil, true, false
					}
					closed = true
				}
			}
			if closed {
				break
			}
		}
	}
	if !closed || inStr || inLit {
		return nil, true, false
	}
	arr, ok := parseTomlStringArray(buf.String())
	return arr, true, ok
}

func parseTomlStringArray(raw string) ([]any, bool) {
	s := strings.TrimSpace(raw)
	if len(s) < 2 || s[0] != '[' || s[len(s)-1] != ']' {
		return nil, false
	}
	i := 1
	end := len(s) - 1
	skipWS := func() {
		for i < end && (s[i] == ' ' || s[i] == '\t' || s[i] == '\r' || s[i] == '\n') {
			i++
		}
	}
	var out []any
	for {
		skipWS()
		if i >= end {
			return out, true
		}
		var val string
		switch s[i] {
		case '"':
			j := i + 1
			esc := false
			for j < end {
				c := s[j]
				if esc {
					esc = false
					j++
					continue
				}
				if c == '\\' {
					esc = true
					j++
					continue
				}
				if c == '"' {
					break
				}
				j++
			}
			if j >= end || s[j] != '"' {
				return nil, false
			}
			u, err := strconv.Unquote(s[i : j+1])
			if err != nil {
				return nil, false
			}
			val = u
			i = j + 1
		case '\'':
			j := i + 1
			for j < end && s[j] != '\'' {
				j++
			}
			if j >= end || s[j] != '\'' {
				return nil, false
			}
			val = s[i+1 : j]
			i = j + 1
		default:
			return nil, false
		}
		out = append(out, val)
		skipWS()
		if i >= end {
			return out, true
		}
		if s[i] != ',' {
			return nil, false
		}
		i++
	}
}

func tomlBlockHasOnlyKeys(blk string, allowed map[string]bool) bool {
	lines := strings.Split(blk, "\n")
	if len(lines) == 0 {
		return false
	}
	for i := 1; i < len(lines); i++ {
		trimmed := strings.TrimSpace(lines[i])
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		if strings.HasPrefix(trimmed, "[") {
			return false
		}
		eq := strings.IndexByte(trimmed, '=')
		if eq <= 0 {
			return false
		}
		key := strings.Trim(strings.TrimSpace(trimmed[:eq]), `"'`)
		if !allowed[key] {
			return false
		}
	}
	return true
}

// IsToklessManagedTomlMcpBlock reports whether the TOML [header] table represents
// a Tokless-managed MCP entry (or, when allowDirectTool != "", a bare legacy direct tool with no custom fields).
func IsToklessManagedTomlMcpBlock(src, header, allowDirectTool string) bool {
	blk, ok := BlockText(src, header)
	if !ok {
		return false
	}
	cmd := TomlBlockField(src, header, "command")
	if cmd == "" {
		return false
	}
	entry := NewOrderedMap()
	entry.Set("command", cmd)
	if args, found, valid := parseTomlStringArrayInBlock(blk, "args"); found {
		if !valid {
			return false
		}
		entry.Set("args", args)
	}
	if IsToklessManagedMcpEntry(entry) {
		return true
	}
	if allowDirectTool != "" && strings.EqualFold(cmd, allowDirectTool) && !strings.ContainsAny(cmd, `/\`) {
		if v, hasArgs := entry.Get("args"); !hasArgs || len(v.([]any)) == 0 {
			return tomlBlockHasOnlyKeys(blk, map[string]bool{
				"command":                     true,
				"enabled":                     true,
				"args":                        true,
				"default_tools_approval_mode": true,
			})
		}
	}
	return false
}
