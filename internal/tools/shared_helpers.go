package tools

import (
	"path/filepath"
	"strings"

	"github.com/HoangP8/tokless/internal/agents"
	"github.com/HoangP8/tokless/internal/util"
)

func kiloInstructionsPath() string { return agents.KiloInstructionsPath() }

func kiloWriteOwner(owner string) bool {
	path := kiloInstructionsPath()
	if path == "" {
		return false
	}
	_ = util.EnsureDir(filepath.Dir(path))
	cur, _ := util.ReadFileSafe(path)
	return writeOwnerInPath(path, cur, owner)
}

func kiloRemoveOwner(owner string) {
	path := kiloInstructionsPath()
	if path == "" {
		return
	}
	if cur, ok := util.ReadFileSafe(path); ok {
		removeOwnerInPath(path, cur, owner)
	}
}

func kiloHasOwner(owner string) bool {
	path := kiloInstructionsPath()
	return path != "" && hasOwnerAtPath(path, owner)
}

func claudeMcpBounded(toolID string) bool {
	raw, ok := util.ReadFileSafe(util.ClaudeCodePaths().GlobalJSON)
	if !ok {
		return false
	}
	cfg := util.TryParseJsonc(raw)
	if cfg == nil {
		return false
	}
	if s, ok := cfg.Get("mcpServers"); ok {
		if sm, ok := s.(*util.OrderedMap); ok {
			v, has := sm.Get(toolID)
			em, isMap := v.(*util.OrderedMap)
			if !has || !isMap {
				return false
			}
			spawn := util.McpSpawnForAgent("claude", toolID)
			cmd, _ := em.Get("command")
			args, _ := em.Get("args")
			return cmd == spawn.Command && util.ArgsEqual(args, spawn.Args)
		}
	}
	return false
}

func openCodeV2PluginsDirIsSeparate() bool {
	return util.OpenCodePathsV2().PluginsDir != util.OpenCodePathsResolved().PluginsDir
}

func toklessOwnedPluginFile(path, marker string) bool {
	raw, ok := util.ReadFileSafe(path)
	return ok && strings.Contains(raw, marker)
}
