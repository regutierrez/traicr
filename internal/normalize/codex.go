package normalize

import (
	"context"
	"io/fs"

	"github.com/regutierrez/traicr/internal/domain"
)

func normalizeCodex(ctx context.Context, adapter string, sourceFS fs.FS) (domain.Normalization, error) {
	if adapter == "codex-rollout-jsonl" {
		return normalizeCodexRollout(ctx, sourceFS)
	}
	thread, err := readJSON(sourceFS, "source/thread.json")
	if err != nil {
		return domain.Normalization{}, err
	}
	if wrapped := object(thread["thread"]); wrapped != nil {
		thread = wrapped
	}
	var events []domain.Event
	var warnings []domain.Warning
	for _, turn := range objects(thread["turns"]) {
		if err := ctx.Err(); err != nil {
			return domain.Normalization{}, err
		}
		turnID := stringValue(turn["id"])
		for _, item := range objects(turn["items"]) {
			if err := ctx.Err(); err != nil {
				return domain.Normalization{}, err
			}
			event, ok := codexItem(item, turnID, "source/thread.json", 0)
			if !ok {
				warnings = append(warnings, warning("unsupported_record", "source/thread.json: unknown Codex item "+stringValue(item["type"])))
				continue
			}
			events = append(events, event)
			if event.Kind == "tool_call" {
				if output := firstValue(item, "output", "result", "error"); output != nil {
					result := event
					result.Kind, result.Text = "tool_result", contentText(output)
					result.Key = nativeKey(result.Kind, stringValue(item["id"]), item)
					events = append(events, result)
				}
			}
		}
		if stringValue(turn["itemsView"]) == "notLoaded" {
			warnings = append(warnings, warning("incomplete_turn", "Codex turn "+turnID+" does not include its items"))
		}
	}
	if len(objects(thread["turns"])) == 0 {
		warnings = append(warnings, warning("unsupported_version", "Codex thread/read result has no turns"))
	}
	return complete(events, warnings), nil
}

func codexItem(item map[string]any, turnID, path string, line int) (domain.Event, bool) {
	kind := stringValue(item["type"])
	id := stringValue(item["id"])
	event := domain.Event{Branch: turnID, Sources: source(path, line)}
	switch kind {
	case "userMessage":
		event.Kind, event.Role, event.Text = "message", "user", contentText(firstValue(item, "content", "text"))
	case "hookPrompt":
		event.Kind, event.Role, event.Text = "message", "system", contentText(firstValue(item, "content", "text"))
	case "agentMessage":
		event.Kind, event.Role, event.Text = "message", "assistant", contentText(firstValue(item, "content", "text"))
	case "reasoning":
		event.Kind, event.Role, event.Text = "reasoning", "assistant", contentText(firstValue(item, "summary", "content", "text"))
	case "functionCall", "commandExecution", "mcpToolCall", "dynamicToolCall", "collabAgentToolCall":
		event.Kind, event.Tool = "tool_call", firstString(item, "name", "tool", "command")
		event.CallID = firstString(item, "callId", "call_id", "id")
		event.Text = readableJSON(firstValue(item, "arguments", "input", "command"))
	case "functionCallOutput":
		event.Kind, event.Tool = "tool_result", firstString(item, "name", "tool")
		event.CallID = firstString(item, "callId", "call_id", "id")
		event.Text = contentText(firstValue(item, "output", "content"))
	case "fileChange":
		event.Kind, event.Text = "file_change", contentText(firstValue(item, "changes", "diff"))
	case "plan", "contextCompaction":
		event.Kind, event.Text = kind, contentText(item)
	default:
		return domain.Event{}, false
	}
	event.Key = nativeKey(event.Kind, id, map[string]any{"turn": turnID, "item": item})
	return event, true
}

func normalizeCodexRollout(ctx context.Context, sourceFS fs.FS) (domain.Normalization, error) {
	records, warnings, err := readJSONL(sourceFS, "source/records.jsonl")
	if err != nil {
		return domain.Normalization{}, err
	}
	var events []domain.Event
	model := ""
	for _, record := range records {
		if err := ctx.Err(); err != nil {
			return domain.Normalization{}, err
		}
		value := record.Value
		kind := stringValue(value["type"])
		payload := object(value["payload"])
		if kind == "turn_context" {
			model = stringValue(payload["model"])
			event := domain.Event{Kind: "model_change", Model: model, Text: model, Timestamp: timestamp(value["timestamp"]), Sources: source(record.Path, record.Line)}
			event.Key = nativeKey(event.Kind, stringValue(payload["id"]), value)
			events = append(events, event)
			continue
		}
		if kind == "response_item" {
			event, ok := codexRolloutItem(payload, model, record)
			if ok {
				events = append(events, event)
			} else {
				warnings = append(warnings, unknown(record, "unsupported Codex response item"))
			}
			continue
		}
		if kind == "compacted" {
			event := domain.Event{Kind: "compaction", Model: model, Text: contentText(payload), Timestamp: timestamp(value["timestamp"]), Sources: source(record.Path, record.Line)}
			event.Key = nativeKey(event.Kind, stringValue(payload["id"]), value)
			events = append(events, event)
			continue
		}
		if kind != "session_meta" {
			warnings = append(warnings, unknown(record, "unsupported Codex rollout record "+kind))
		}
	}
	return complete(events, warnings), nil
}

func codexRolloutItem(payload map[string]any, model string, record sourceRecord) (domain.Event, bool) {
	typeName := stringValue(payload["type"])
	event := domain.Event{Model: model, Timestamp: timestamp(record.Value["timestamp"]), Sources: source(record.Path, record.Line)}
	switch typeName {
	case "message":
		event.Kind, event.Role, event.Text = "message", stringValue(payload["role"]), contentText(payload["content"])
	case "reasoning":
		event.Kind, event.Role, event.Text = "reasoning", "assistant", contentText(payload["summary"])
	case "function_call":
		event.Kind, event.Tool, event.CallID = "tool_call", stringValue(payload["name"]), stringValue(payload["call_id"])
		event.Text = contentText(payload["arguments"])
	case "function_call_output":
		event.Kind, event.CallID, event.Text = "tool_result", stringValue(payload["call_id"]), contentText(payload["output"])
	default:
		return domain.Event{}, false
	}
	event.Key = nativeKey(event.Kind, stringValue(payload["id"]), payload)
	return event, true
}
