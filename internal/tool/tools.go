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
func SubAgentTools() []Tool {
	return []Tool{
		&BashTool{},
		&ReadTool{},
		&WriteTool{},
		&EditTool{},
		&GlobTool{},
		&GrepTool{},
	}
}
