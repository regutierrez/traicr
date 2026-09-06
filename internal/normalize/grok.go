package normalize

import (
	"context"
	"io/fs"

	"github.com/regutierrez/traicr/internal/domain"
)

func normalizeGrok(ctx context.Context, sourceFS fs.FS) (domain.Normalization, error) {
	summary, err := readJSON(sourceFS, "source/summary.json")
	if err != nil {
		return domain.Normalization{}, err
	}
	version := numberString(summary["chat_format_version"])
	if version != "" && version != "1" {
		return complete(nil, []domain.Warning{warning("unsupported_version", "Grok Build chat_format_version "+version+" is not supported")}), nil
	}
	records, warnings, err := readJSONL(sourceFS, "source/updates.jsonl")
	if err != nil {
		return domain.Normalization{}, err
	}
	var events []domain.Event
	for _, record := range records {
		if err := ctx.Err(); err != nil {
			return domain.Normalization{}, err
		}
		value := record.Value
		params := object(value["params"])
		if params == nil {
			params = value
		}
		update := object(params["update"])
		if update == nil {
			update = object(value["update"])
		}
		kind := stringValue(update["sessionUpdate"])
		if kind == "" {
			kind = stringValue(update["session_update"])
		}
		meta := object(params["_meta"])
		id := firstString(meta, "eventId", "event_id")
		event := domain.Event{Timestamp: timestamp(value["timestamp"]), Sources: source(record.Path, record.Line)}
		switch kind {
		case "user_message_chunk":
			event.Kind, event.Role, event.Text = "message", "user", contentText(update["content"])
		case "agent_message_chunk":
			event.Kind, event.Role, event.Text = "message", "assistant", contentText(update["content"])
		case "agent_thought_chunk":
			event.Kind, event.Role, event.Text = "reasoning", "assistant", contentText(update["content"])
		case "tool_call":
			event.Kind, event.Tool = "tool_call", firstString(update, "title", "name", "kind")
			event.CallID = firstString(update, "toolCallId", "tool_call_id")
			event.Text = readableJSON(update["input"])
		case "tool_call_update":
			event.Kind, event.CallID = "tool_update", firstString(update, "toolCallId", "tool_call_id")
			event.Text = contentText(firstValue(update, "content", "rawOutput", "output"))
			if status := stringValue(update["status"]); status == "completed" || status == "failed" {
				event.Kind = "tool_result"
			}
		case "model_changed", "model_auto_switched":
			event.Kind = "model_change"
			event.Model = firstString(update, "modelId", "model_id", "model")
			event.Text = event.Model
		case "auto_compact_started", "auto_compact_completed", "compaction_checkpoint", "session_summary_generated", "session_recap":
			event.Kind, event.Text = "compaction", contentText(update)
		case "subagent_spawned", "subagent_progress", "subagent_finished", "rewind_marker", "turn_completed", "task_completed":
			event.Kind, event.Text = kind, contentText(update)
			event.Branch = firstString(update, "child_session_id", "subagent_id", "parent_prompt_id")
		default:
			warnings = append(warnings, unknown(record, "unsupported Grok Build update "+kind))
			continue
		}
		event.Key = nativeKey(event.Kind, id, update)
		events = append(events, event)
	}
	return complete(events, warnings), nil
}
