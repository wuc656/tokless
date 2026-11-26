package tools

import (
	"github.com/HoangP8/tokless/internal/agents"
	"github.com/HoangP8/tokless/internal/core"
	"github.com/HoangP8/tokless/internal/util"
)

var instructionHarnessIDs = []string{
	"claude", "opencode", "opencode2", "codex", "antigravity",
	"copilot", "droid", "grok", "pi", "omp",
	"kilo", "cline", "amp", "kimi", "dsh", "zcode",
}

func newInstructionTool(spec *core.ToolManifest) *core.ToolManifest {
	spec.InstructionOnly = true
	spec.NotTrackable = true
	spec.Channel = core.ChannelGitHub
	if spec.Install == nil {
		spec.Install = func(opts core.RunOpts) (bool, error) {
			opts.Reportf("instruction-only", 1)
			return true, nil
		}
	}
	spec.WireFor = make(map[string]core.AgentFn, len(instructionHarnessIDs))
	spec.UnwireFor = make(map[string]core.AgentFn, len(instructionHarnessIDs))
	spec.VerifyFor = make(map[string]core.VerifyFn, len(instructionHarnessIDs))

	owner := spec.ID
	for _, agentID := range instructionHarnessIDs {
		id := agentID
		switch id {
		case "kilo":
			spec.WireFor[id] = func(opts core.RunOpts) (bool, error) {
				if opts.DryRun {
					util.L.Sub("[dry-run] would add " + owner + " section to kilo")
					return true, nil
				}
				kiloWriteOwner(owner)
				return kiloHasOwner(owner), nil
			}
			spec.UnwireFor[id] = func(opts core.RunOpts) (bool, error) {
				if opts.DryRun {
					return true, nil
				}
				kiloRemoveOwner(owner)
				return true, nil
			}
			spec.VerifyFor[id] = func() *bool {
				return core.BoolPtr(kiloHasOwner(owner))
			}
		case "copilot":
			spec.WireFor[id] = func(opts core.RunOpts) (bool, error) {
				if opts.DryRun {
					util.L.Sub("[dry-run] would add " + owner + " section to copilot")
					return true, nil
				}
				WriteOwner("copilot", owner)
				if HasOwner("copilot", owner) {
					agents.SyncCopilotIdeInstructions()
				}
				return HasOwner("copilot", owner), nil
			}
			spec.UnwireFor[id] = func(opts core.RunOpts) (bool, error) {
				if opts.DryRun {
					return true, nil
				}
				RemoveOwner("copilot", owner)
				agents.SyncCopilotIdeInstructions()
				return true, nil
			}
			spec.VerifyFor[id] = func() *bool {
				return core.BoolPtr(HasOwner("copilot", owner))
			}
		default:
			spec.WireFor[id] = func(opts core.RunOpts) (bool, error) {
				if opts.DryRun {
					util.L.Sub("[dry-run] would add " + owner + " section to " + id)
					return true, nil
				}
				WriteOwner(id, owner)
				return HasOwner(id, owner), nil
			}
			spec.UnwireFor[id] = func(opts core.RunOpts) (bool, error) {
				if opts.DryRun {
					return true, nil
				}
				RemoveOwner(id, owner)
				return true, nil
			}
			spec.VerifyFor[id] = func() *bool {
				return core.BoolPtr(HasOwner(id, owner))
			}
		}
	}
	return spec
}

var principles = newInstructionTool(&core.ToolManifest{
	ID:          "principles",
	Label:       "Karpathy Principles",
	Description: "Low-interference surgical coding discipline without unrequested test generation.",
	Homepage:    "https://github.com/multica-ai/andrej-karpathy-skills",
	InstallHint: "Instruction-only — managed by tokless.",
})

var caveman = newInstructionTool(&core.ToolManifest{
	ID:          "caveman",
	Label:       "Caveman",
	Description: "Terse user-facing output compression without altering code quality or reasoning.",
	Homepage:    "https://github.com/JuliusBrussee/caveman",
	InstallHint: "Instruction-only — managed by tokless.",
})

var ponytail = newInstructionTool(&core.ToolManifest{
	ID:          "ponytail",
	Label:       "Ponytail",
	Description: "Minimal-diff lite discipline: reuse existing helpers and avoid speculative abstractions.",
	Homepage:    "https://github.com/DietrichGebert/ponytail",
	InstallHint: "Instruction-only — managed by tokless.",
})
