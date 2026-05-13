package service

import (
	"fmt"
	"strconv"
	"strings"
)

type PortRule struct {
	ServerPorts []int
	LocalPorts  []int
}

func ParsePorts(s string) ([]PortRule, error) {
	if s == "" {
		return nil, fmt.Errorf("empty port mapping")
	}

	parts := strings.SplitN(s, ":", 2)
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid port mapping format: %q (expected server_ports:local_ports)", s)
	}

	serverPorts, err := parsePortSpec(parts[0])
	if err != nil {
		return nil, fmt.Errorf("server ports: %w", err)
	}

	localPorts, err := parsePortSpec(parts[1])
	if err != nil {
		return nil, fmt.Errorf("local ports: %w", err)
	}

	n, m := len(serverPorts), len(localPorts)
	var result []PortRule

	switch {
	case n == 1 && m > 1:
		for _, lp := range localPorts {
			result = append(result, PortRule{
				ServerPorts: []int{serverPorts[0]},
				LocalPorts:  []int{lp},
			})
		}
	case m == 1 && n > 1:
		for _, sp := range serverPorts {
			result = append(result, PortRule{
				ServerPorts: []int{sp},
				LocalPorts:  []int{localPorts[0]},
			})
		}
	default:
		for i := 0; i < n; i++ {
			lpIdx := i
			if lpIdx >= m {
				lpIdx = m - 1
			}
			result = append(result, PortRule{
				ServerPorts: []int{serverPorts[i]},
				LocalPorts:  []int{localPorts[lpIdx]},
			})
		}
	}

	return result, nil
}

func parsePortSpec(s string) ([]int, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil, fmt.Errorf("empty port spec")
	}

	parts := strings.Split(s, ",")
	var result []int

	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}

		if strings.Contains(part, "-") {
			ports, err := expandRange(part)
			if err != nil {
				return nil, err
			}
			result = append(result, ports...)
		} else {
			port, err := strconv.Atoi(part)
			if err != nil {
				return nil, fmt.Errorf("invalid port %q: %w", part, err)
			}
			if port < 1 || port > 65535 {
				return nil, fmt.Errorf("port %d out of range (1-65535)", port)
			}
			result = append(result, port)
		}
	}

	if len(result) == 0 {
		return nil, fmt.Errorf("empty port list after parsing")
	}

	return result, nil
}

func expandRange(s string) ([]int, error) {
	parts := strings.SplitN(s, "-", 2)
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid range: %q", s)
	}

	startStr := strings.TrimSpace(parts[0])
	endStr := strings.TrimSpace(parts[1])

	if startStr == "" || endStr == "" {
		return nil, fmt.Errorf("invalid range: %q", s)
	}

	start, err := strconv.Atoi(startStr)
	if err != nil {
		return nil, fmt.Errorf("invalid range start %q: %w", startStr, err)
	}

	end, err := strconv.Atoi(endStr)
	if err != nil {
		return nil, fmt.Errorf("invalid range end %q: %w", endStr, err)
	}

	if start < 1 || start > 65535 || end < 1 || end > 65535 {
		return nil, fmt.Errorf("port range %d-%d out of range (1-65535)", start, end)
	}

	if start > end {
		return nil, fmt.Errorf("invalid range %d-%d (start > end)", start, end)
	}

	var result []int
	for i := start; i <= end; i++ {
		result = append(result, i)
	}

	return result, nil
}
