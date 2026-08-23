package main

import (
	"hostops/pfm/internal/action"
	"hostops/pfm/internal/ask"
	pfmengine "hostops/pfm/internal/engine"
	claudeengine "hostops/pfm/internal/engine/claude"
	codexengine "hostops/pfm/internal/engine/codex"
	opencodeengine "hostops/pfm/internal/engine/opencode"
	"hostops/pfm/internal/gather"
	"hostops/pfm/internal/index"
	"hostops/pfm/internal/spawn"
	"hostops/pfm/internal/stats"
)

// registerEngines is the single composition root for engine capabilities.
func registerEngines() {
	index.RegisterSource(pfmengine.Claude, claudeengine.Source{})
	index.RegisterSource(pfmengine.Codex, codexengine.Source{})
	index.RegisterSource(pfmengine.Opencode, opencodeengine.Source{})

	spawn.RegisterLauncher(pfmengine.Claude, claudeengine.Launcher{})
	spawn.RegisterLauncher(pfmengine.Codex, codexengine.Launcher{})

	gather.RegisterMatcher(pfmengine.Claude, claudeengine.Matcher{})
	gather.RegisterMatcher(pfmengine.Codex, codexengine.Matcher{})
	gather.RegisterMatcher(pfmengine.Opencode, opencodeengine.Matcher{})

	stats.RegisterUsageSource(pfmengine.Claude, claudeengine.UsageSource{})
	stats.RegisterUsageSource(pfmengine.Codex, codexengine.UsageSource{})

	action.RegisterPlanner(pfmengine.Claude, claudeengine.HeadlessPlanner{})
	action.RegisterPlanner(pfmengine.Codex, codexengine.HeadlessPlanner{})

	ask.RegisterRunner(pfmengine.Claude, claudeengine.AskRunner{})
	ask.RegisterRunner(pfmengine.Codex, codexengine.AskRunner{})
}

func init() { registerEngines() }
