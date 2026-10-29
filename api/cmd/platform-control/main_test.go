package main

import (
	"strings"
	"testing"
)

func TestParseOptionsRejectsUnsafeMutationInputs(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want string
	}{
		{name: "missing platform", args: []string{"--state", "fully_disabled", "--expected-version", "1", "--reason", "security_incident"}, want: "platform"},
		{name: "unknown platform", args: []string{"--platform", "myspace", "--state", "fully_disabled", "--expected-version", "1", "--reason", "security_incident"}, want: "unsupported platform"},
		{name: "unsupported state", args: []string{"--platform", "tiktok", "--state", "paused", "--expected-version", "1", "--reason", "security_incident"}, want: "state"},
		{name: "missing expected version", args: []string{"--platform", "tiktok", "--state", "fully_disabled", "--reason", "security_incident"}, want: "expected-version"},
		{name: "missing reason", args: []string{"--platform", "tiktok", "--state", "fully_disabled", "--expected-version", "1"}, want: "reason"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := parseOptions(test.args)
			if err == nil || !strings.Contains(strings.ToLower(err.Error()), test.want) {
				t.Fatalf("error=%v, want containing %q", err, test.want)
			}
		})
	}
}

func TestParseOptionsAcceptsExplicitTikTokDisable(t *testing.T) {
	options, err := parseOptions([]string{
		"--platform", "TikTok",
		"--state", "fully_disabled",
		"--expected-version", "2",
		"--reason", "security_incident",
	})
	if err != nil {
		t.Fatal(err)
	}
	if options.platform != "tiktok" || options.state != "fully_disabled" || options.expectedVersion != 2 || options.reason != "security_incident" {
		t.Fatalf("options=%+v", options)
	}
}
