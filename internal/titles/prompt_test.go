package titles

import (
	"strings"
	"testing"

	"github.com/regutierrez/traicr/internal/store"
)

func TestCleanMessageKeepsOnlyWhatTheUserWrote(t *testing.T) {
	for _, test := range []struct{ name, in, want string }{
		{"cursor agent query", "<timestamp>Monday, Sep 28, 2026, 10:50 PM (UTC-4)</timestamp>\n<user_query>\nWhy does the parent agent's Shell tool fail?\n</user_query>", "Why does the parent agent's Shell tool fail?"},
		{"pi skill invocation", "<skill name=\"review\" location=\"/x\">\nLong skill body\n</skill>\n\nreview my Go changes", "review my Go changes"},
		{"injected context", "<manually_attached_skills>a</manually_attached_skills><mcp_meta_tools>b</mcp_meta_tools>add a CLI", "add a CLI"},
		{"claude command", "<command-name>/review</command-name>\n<command-args>the auth PR</command-args>", "/review the auth PR"},
		{"plain", "  fix   the\n\nbuild  ", "fix the build"},
		{"only injected", "<timestamp>now</timestamp>", ""},
		{"block cut off by a length limit", "<skill name=\"x\">\nlong skill body that was trunc", ""},
	} {
		if got := cleanMessage(test.in); got != test.want {
			t.Errorf("%s: cleanMessage = %q, want %q", test.name, got, test.want)
		}
	}
}

func TestPromptSkipsTracesWithoutUserTextAndKeepsBothEnds(t *testing.T) {
	if got := Prompt(store.TitleSource{Harness: "pi", UserMessages: []string{"<timestamp>x</timestamp>"}, LastAssistantMessage: "done"}); got != "" {
		t.Fatalf("prompt without user text = %q", got)
	}
	messages := []string{"first: migrate the collector to Go 1.27"}
	for range 40 {
		messages = append(messages, strings.Repeat("middle ", 250))
	}
	messages = append(messages, "last: now publish the release")
	prompt := Prompt(store.TitleSource{Harness: "pi", Repository: "github.com/regutierrez/traicr", WorkingDirectory: "/home/pael/traicr", UserMessages: messages, LastAssistantMessage: "Released v0.2.0."})
	if !strings.HasPrefix(prompt, "Name this archived coding-agent session.") || !strings.HasSuffix(prompt, "SESSION EXCERPT>>>\n\nReply with only the title for the session above, at most 60 characters.") {
		t.Errorf("prompt does not quote the excerpt and restate the task after it:\n%s", prompt)
	}
	for _, want := range []string{"Harness: pi", "Repository: github.com/regutierrez/traicr", "User message 1:\nfirst: migrate", "user messages omitted]", "User message 42:\nlast: now publish", "Final assistant message:\nReleased v0.2.0."} {
		if !strings.Contains(prompt, want) {
			t.Errorf("prompt missing %q", want)
		}
	}
	if n := len([]rune(prompt)); n > maxPromptRunes+2000 {
		t.Fatalf("prompt has %d runes, want about %d", n, maxPromptRunes)
	}
}

func TestCleanTitle(t *testing.T) {
	for in, want := range map[string]string{
		"Speed up incremental collect":                                    "Speed up incremental collect",
		"\n\"Fix login redirect loop.\"\n":                                "Fix login redirect loop",
		"Title: **Add CLI rename command**":                               "Add CLI rename command",
		"<think>about it</think>\nMigrate store to v3":                    "Migrate store to v3",
		"`Debug cursor-agent`\nExtra explanation":                         "Debug cursor-agent",
		"<longcat_tool_call>Bash":                                         "",
		"I'll explore the workspace first to understand the Svelte setup": "",
		"Let me check the plugin":                                         "",
		strings.Repeat("prose ", 20):                                      "",
		"":                                                                "",
		"   \n  ":                                                         "",
		strings.Repeat("word ", 18):                                       strings.TrimSpace(strings.Repeat("word ", 16)),
	} {
		if got := cleanTitle(in); got != want {
			t.Errorf("cleanTitle(%q) = %q, want %q", in, got, want)
		}
	}
}
