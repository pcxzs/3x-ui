package tgbot

import "testing"

// A stranger must reach nothing but the greeting: any command that leaks
// through tells them the bot does more than say hello.
func TestCommandAllowedByLevel(t *testing.T) {
	tests := []struct {
		level   userLevel
		command string
		want    bool
	}{
		{levelStranger, "start", true},
		{levelStranger, "help", false},
		{levelStranger, "usage", false},
		{levelStranger, "pm", false},
		{levelStranger, "clients", false},
		{levelStranger, "server", false},
		{levelStranger, "broadcast", false},

		{levelClient, "start", true},
		{levelClient, "help", true},
		{levelClient, "usage", true},
		{levelClient, "pm", true},
		{levelClient, "clients", false},
		{levelClient, "server", false},
		{levelClient, "broadcast", false},
		{levelClient, "sethelp", false},
		{levelClient, "whois", false},

		{levelAdmin, "start", true},
		{levelAdmin, "clients", true},
		{levelAdmin, "server", true},
		{levelAdmin, "broadcast", true},
	}

	for _, tc := range tests {
		if got := commandAllowed(tc.level, tc.command); got != tc.want {
			t.Errorf("commandAllowed(level %d, %q) = %v, want %v", tc.level, tc.command, got, tc.want)
		}
	}
}

// Every admin-only command reachable from the router must be denied to the two
// lower levels, so adding one without listing it cannot silently expose it.
func TestAdminCommandsAreNeverAllowedBelowAdmin(t *testing.T) {
	adminOnly := []string{
		"sethelp", "broadcast", "send", "whois", "clients", "server",
		"inbound", "restart", "clearall",
	}
	for _, command := range adminOnly {
		for _, level := range []userLevel{levelStranger, levelClient} {
			if commandAllowed(level, command) {
				t.Errorf("%q must not be allowed at level %d", command, level)
			}
		}
	}
}
