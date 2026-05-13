package port

// RuleEngine defines the AutoProxy rule matching interface.
type RuleEngine interface {
	// Match determines whether a target should go through the proxy.
	// target is "host:port" format.
	// Returns: proxy=true means go through SSH tunnel, proxy=false means direct connect.
	Match(target string) (proxy bool, rule *MatchedRule, err error)

	// Load loads rules from a file path.
	Load(path string) error

	// Reload reloads rules from the previously loaded file.
	Reload() error

	// DefaultAction returns whether unmatched targets default to proxy (true) or direct (false).
	DefaultAction() bool
}

// MatchedRule contains information about the matched rule.
type MatchedRule struct {
	Pattern string // The matching pattern (e.g. "*.google.com")
	Action  string // "proxy" or "direct"
	Line    int    // Line number in rules file
}
