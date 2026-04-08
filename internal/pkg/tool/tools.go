package tool

// DefaultTools returns all tools available in the interactive REPL.
func DefaultTools() []Tool {
	return []Tool{
		&BashTool{},
		&ReadTool{},
		&WriteTool{},
		&EditTool{},
		&GlobTool{},
		&GrepTool{},
	}
}

// SubAgentTools returns tools safe for non-interactive sub-agents.
// Currently identical to DefaultTools; will diverge when interactive tools
// (e.g. AskUserTool) are added to DefaultTools.
func SubAgentTools() []Tool {
	return DefaultTools()
}
