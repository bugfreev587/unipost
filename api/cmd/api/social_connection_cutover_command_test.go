package main

import (
	"testing"
	"time"
)

func TestParseSocialConnectionCommands(t *testing.T) {
	tests := []struct {
		args    []string
		command string
		mode    string
		json    bool
		timeout time.Duration
	}{
		{args: []string{"api", "social-connections-preflight", "--mode=expand"}, command: "preflight", mode: "expand"},
		{args: []string{"api", "social-connections-preflight", "--mode=cutover", "--json"}, command: "preflight", mode: "cutover", json: true},
		{args: []string{"api", "social-connections-cutover", "--json", "--drain-timeout=7m"}, command: "cutover", mode: "cutover", json: true, timeout: 7 * time.Minute},
	}
	for _, test := range tests {
		options, handled, err := parseSocialConnectionCommand(test.args)
		if err != nil || !handled {
			t.Fatalf("parse %v = handled:%t error:%v", test.args, handled, err)
		}
		if options.Command != test.command || options.Mode != test.mode || options.JSON != test.json {
			t.Fatalf("parse %v = %+v", test.args, options)
		}
		if test.timeout != 0 && options.DrainTimeout != test.timeout {
			t.Fatalf("timeout = %v, want %v", options.DrainTimeout, test.timeout)
		}
	}
}

func TestParseSocialConnectionCommandRejectsUnknownFlags(t *testing.T) {
	for _, args := range [][]string{
		{"api", "social-connections-preflight", "--unknown"},
		{"api", "social-connections-preflight", "--mode=invalid"},
		{"api", "social-connections-cutover", "--confirm-old-pods-drained"},
		{"api", "social-connections-cutover", "--drain-timeout=0s"},
	} {
		if _, handled, err := parseSocialConnectionCommand(args); !handled || err == nil {
			t.Fatalf("parse %v = handled:%t error:%v, want rejection", args, handled, err)
		}
	}
}
