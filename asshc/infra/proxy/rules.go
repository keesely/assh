package proxy

import (
	"bufio"
	"fmt"
	"net"
	"os"
	"regexp"
	"strings"
	"sync"

	"assh/asshc/port"
)

// ruleEngine implements port.RuleEngine for AutoProxy-compatible rule files.
type ruleEngine struct {
	mu           sync.RWMutex
	rules        []*parsedRule
	ruleFilePath string
	defaultProxy bool
}

// parsedRule represents a single parsed rule from the rules file.
type parsedRule struct {
	pattern      string
	action       string
	line         int
	isWhite      bool
	isRegex      bool
	regex        *regexp.Regexp
	domainSuffix string
	exactDomain  string
	cidr         *net.IPNet
}

// NewRuleEngine creates a rule engine with the given default action.
// defaultProxy=true means unmatched targets go through the proxy.
func NewRuleEngine(defaultProxy bool) *ruleEngine {
	return &ruleEngine{
		defaultProxy: defaultProxy,
	}
}

// Match determines whether a target should go through the proxy.
// target is in "host:port" format.
func (e *ruleEngine) Match(target string) (bool, *port.MatchedRule, error) {
	host, _, err := net.SplitHostPort(target)
	if err != nil {
		host = target
	}
	host = strings.ToLower(host)

	e.mu.RLock()
	defer e.mu.RUnlock()

	// Priority 1: All whitelist rules (any type)
	for _, r := range e.rules {
		if r.isWhite && matchRule(host, r) {
			return false, toMatchedRule(r), nil
		}
	}

	isIP := net.ParseIP(host) != nil

	// Priority 2: CIDR (non-whitelist, IP only)
	if isIP {
		parsedIP := net.ParseIP(host)
		for _, r := range e.rules {
			if !r.isWhite && r.cidr != nil && r.cidr.Contains(parsedIP) {
				return true, toMatchedRule(r), nil
			}
		}
	}

	// Priority 3: Regex (non-whitelist)
	for _, r := range e.rules {
		if !r.isWhite && r.isRegex && r.regex.MatchString(host) {
			return true, toMatchedRule(r), nil
		}
	}

	// Priority 4: Domain suffix (non-whitelist)
	for _, r := range e.rules {
		if !r.isWhite && r.domainSuffix != "" && strings.HasSuffix(host, r.domainSuffix) {
			return true, toMatchedRule(r), nil
		}
	}

	// Priority 5: Exact domain (non-whitelist)
	for _, r := range e.rules {
		if !r.isWhite && r.exactDomain != "" && host == r.exactDomain {
			return true, toMatchedRule(r), nil
		}
	}

	return e.defaultProxy, nil, nil
}

// Load loads rules from a file path.
func (e *ruleEngine) Load(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("open rules file %s: %w", path, err)
	}
	defer f.Close()

	var rules []*parsedRule
	scanner := bufio.NewScanner(f)
	lineNum := 0
	for scanner.Scan() {
		lineNum++
		line := strings.TrimSpace(scanner.Text())

		if line == "" || strings.HasPrefix(line, "!") {
			continue
		}

		rule, err := parseRule(line, lineNum)
		if err != nil {
			continue
		}
		if rule != nil {
			rules = append(rules, rule)
		}
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("read rules file %s: %w", path, err)
	}

	e.mu.Lock()
	e.rules = rules
	e.ruleFilePath = path
	e.mu.Unlock()

	return nil
}

// Reload reloads rules from the previously loaded file path.
func (e *ruleEngine) Reload() error {
	e.mu.RLock()
	path := e.ruleFilePath
	e.mu.RUnlock()

	if path == "" {
		return fmt.Errorf("no rules file has been loaded")
	}

	return e.Load(path)
}

// DefaultAction returns whether unmatched targets default to proxy (true) or direct (false).
func (e *ruleEngine) DefaultAction() bool {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.defaultProxy
}

// parseRule parses a single rule line into a parsedRule.
func parseRule(line string, lineNum int) (*parsedRule, error) {
	isWhite := strings.HasPrefix(line, "@@")
	if isWhite {
		line = strings.TrimSpace(line[2:])
	}

	if line == "" {
		return nil, nil
	}

	rule := &parsedRule{
		pattern: line,
		line:    lineNum,
		isWhite: isWhite,
		action:  "proxy",
	}
	if isWhite {
		rule.action = "direct"
	}

	// Regex pattern: /pattern/
	if strings.HasPrefix(line, "/") && strings.HasSuffix(line, "/") && len(line) >= 3 {
		pattern := line[1 : len(line)-1]
		re, err := regexp.Compile(pattern)
		if err != nil {
			return nil, fmt.Errorf("line %d: invalid regex %q: %w", lineNum, pattern, err)
		}
		rule.isRegex = true
		rule.regex = re
		return rule, nil
	}

	// CIDR: contains "/"
	if strings.Contains(line, "/") {
		_, ipnet, err := net.ParseCIDR(line)
		if err == nil {
			rule.cidr = ipnet
			return rule, nil
		}
	}

	// Exact IP
	if ip := net.ParseIP(line); ip != nil {
		rule.exactDomain = strings.ToLower(line)
		return rule, nil
	}

	// Domain suffix: *.domain.com
	if strings.HasPrefix(line, "*.") {
		rule.domainSuffix = strings.ToLower(line[1:])
		return rule, nil
	}

	// Default: exact domain
	rule.exactDomain = strings.ToLower(line)
	return rule, nil
}

// matchRule checks if a host matches a parsed rule.
func matchRule(host string, rule *parsedRule) bool {
	switch {
	case rule.cidr != nil:
		ip := net.ParseIP(host)
		if ip == nil {
			return false
		}
		return rule.cidr.Contains(ip)
	case rule.isRegex:
		return rule.regex.MatchString(host)
	case rule.domainSuffix != "":
		return strings.HasSuffix(host, rule.domainSuffix)
	case rule.exactDomain != "":
		return host == rule.exactDomain
	}
	return false
}

// toMatchedRule converts a parsedRule to a port.MatchedRule.
func toMatchedRule(r *parsedRule) *port.MatchedRule {
	return &port.MatchedRule{
		Pattern: r.pattern,
		Action:  r.action,
		Line:    r.line,
	}
}
