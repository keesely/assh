package service

import (
	"reflect"
	"testing"
)

func TestParsePorts_MultiToOne(t *testing.T) {
	rules, err := ParsePorts("80,443:80")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(rules) != 2 {
		t.Fatalf("expected 2 rules, got %d", len(rules))
	}
	expected := []PortRule{
		{ServerPorts: []int{80}, LocalPorts: []int{80}},
		{ServerPorts: []int{443}, LocalPorts: []int{80}},
	}
	if !reflect.DeepEqual(rules, expected) {
		t.Errorf("got %+v, want %+v", rules, expected)
	}
}

func TestParsePorts_OneToMulti(t *testing.T) {
	rules, err := ParsePorts("443:80,443")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(rules) != 2 {
		t.Fatalf("expected 2 rules, got %d", len(rules))
	}
	expected := []PortRule{
		{ServerPorts: []int{443}, LocalPorts: []int{80}},
		{ServerPorts: []int{443}, LocalPorts: []int{443}},
	}
	if !reflect.DeepEqual(rules, expected) {
		t.Errorf("got %+v, want %+v", rules, expected)
	}
}

func TestParsePorts_EqualRange(t *testing.T) {
	rules, err := ParsePorts("80-85:81-86")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(rules) != 6 {
		t.Fatalf("expected 6 rules, got %d", len(rules))
	}
	for i, sp := range []int{80, 81, 82, 83, 84, 85} {
		lp := sp + 1
		if rules[i].ServerPorts[0] != sp || rules[i].LocalPorts[0] != lp {
			t.Errorf("rule[%d]: got %d->%d, want %d->%d", i, rules[i].ServerPorts[0], rules[i].LocalPorts[0], sp, lp)
		}
	}
}

func TestParsePorts_TruncatedRange(t *testing.T) {
	rules, err := ParsePorts("80-88:81-84")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(rules) != 9 {
		t.Fatalf("expected 9 rules, got %d", len(rules))
	}
	for i, sp := range []int{80, 81, 82, 83} {
		lp := sp + 1
		if rules[i].ServerPorts[0] != sp || rules[i].LocalPorts[0] != lp {
			t.Errorf("rule[%d]: got %d->%d, want %d->%d", i, rules[i].ServerPorts[0], rules[i].LocalPorts[0], sp, lp)
		}
	}
	for i := 4; i < 9; i++ {
		if rules[i].LocalPorts[0] != 84 {
			t.Errorf("rule[%d]: got local %d, want 84", i, rules[i].LocalPorts[0])
		}
	}
}

func TestParsePorts_LocalLongerRange(t *testing.T) {
	rules, err := ParsePorts("80-84:81-88")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(rules) != 5 {
		t.Fatalf("expected 5 rules, got %d", len(rules))
	}
	for i, sp := range []int{80, 81, 82, 83, 84} {
		lp := sp + 1
		if rules[i].ServerPorts[0] != sp || rules[i].LocalPorts[0] != lp {
			t.Errorf("rule[%d]: got %d->%d, want %d->%d", i, rules[i].ServerPorts[0], rules[i].LocalPorts[0], sp, lp)
		}
	}
}

func TestParsePorts_MixedSingleAndRange(t *testing.T) {
	rules, err := ParsePorts("443,80-85:80")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(rules) != 7 {
		t.Fatalf("expected 7 rules, got %d", len(rules))
	}
	if rules[0].ServerPorts[0] != 443 || rules[0].LocalPorts[0] != 80 {
		t.Errorf("rule[0]: got %d->%d, want 443->80", rules[0].ServerPorts[0], rules[0].LocalPorts[0])
	}
	for i := 1; i < 7; i++ {
		expectedSP := 79 + i
		if rules[i].ServerPorts[0] != expectedSP || rules[i].LocalPorts[0] != 80 {
			t.Errorf("rule[%d]: got %d->%d, want %d->80", i, rules[i].ServerPorts[0], rules[i].LocalPorts[0], expectedSP)
		}
	}
}

func TestParsePorts_Error_InvalidPort(t *testing.T) {
	_, err := ParsePorts("0:80")
	if err == nil {
		t.Error("expected error for port 0")
	}

	_, err = ParsePorts("80:65536")
	if err == nil {
		t.Error("expected error for port 65536")
	}

	_, err = ParsePorts("80:abc")
	if err == nil {
		t.Error("expected error for non-numeric port")
	}

	_, err = ParsePorts("-1:80")
	if err == nil {
		t.Error("expected error for negative port")
	}
}

func TestParsePorts_Error_InvalidRange(t *testing.T) {
	_, err := ParsePorts("90-80:80")
	if err == nil {
		t.Error("expected error for start > end")
	}
}

func TestParsePorts_Error_NoColon(t *testing.T) {
	_, err := ParsePorts("80,443")
	if err == nil {
		t.Error("expected error for missing colon")
	}
}

func TestParsePorts_Error_Empty(t *testing.T) {
	_, err := ParsePorts("")
	if err == nil {
		t.Error("expected error for empty string")
	}
}

func TestParsePorts_SingleMapping(t *testing.T) {
	rules, err := ParsePorts("80:8080")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(rules) != 1 {
		t.Fatalf("expected 1 rule, got %d", len(rules))
	}
	if rules[0].ServerPorts[0] != 80 || rules[0].LocalPorts[0] != 8080 {
		t.Errorf("got %d->%d, want 80->8080", rules[0].ServerPorts[0], rules[0].LocalPorts[0])
	}
}

func TestParsePortSpec(t *testing.T) {
	ports, err := parsePortSpec("80,443,8000-8005")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := []int{80, 443, 8000, 8001, 8002, 8003, 8004, 8005}
	if !reflect.DeepEqual(ports, expected) {
		t.Errorf("got %v, want %v", ports, expected)
	}
}

func TestParsePortSpec_Single(t *testing.T) {
	ports, err := parsePortSpec("80")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := []int{80}
	if !reflect.DeepEqual(ports, expected) {
		t.Errorf("got %v, want %v", ports, expected)
	}
}

func TestParsePortSpec_Range(t *testing.T) {
	ports, err := parsePortSpec("80-85")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := []int{80, 81, 82, 83, 84, 85}
	if !reflect.DeepEqual(ports, expected) {
		t.Errorf("got %v, want %v", ports, expected)
	}
}

func TestExpandRange(t *testing.T) {
	ports, err := expandRange("80-85")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := []int{80, 81, 82, 83, 84, 85}
	if !reflect.DeepEqual(ports, expected) {
		t.Errorf("got %v, want %v", ports, expected)
	}
}

func TestExpandRange_Single(t *testing.T) {
	ports, err := expandRange("80-80")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(ports) != 1 || ports[0] != 80 {
		t.Errorf("got %v, want [80]", ports)
	}
}

func TestExpandRange_Invalid(t *testing.T) {
	_, err := expandRange("90-80")
	if err == nil {
		t.Error("expected error for start > end")
	}
}

func TestParsePorts_Error_EmptyServerSide(t *testing.T) {
	_, err := ParsePorts(":80")
	if err == nil {
		t.Error("expected error for empty server side")
	}
}

func TestParsePorts_Error_EmptyLocalSide(t *testing.T) {
	_, err := ParsePorts("80:")
	if err == nil {
		t.Error("expected error for empty local side")
	}
}
