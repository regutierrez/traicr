package normalize

import (
	"context"
	"io/fs"

	"github.com/regutierrez/traicr/internal/domain"
)

func normalizeClaude(ctx context.Context, sourceFS fs.FS) (domain.Normalization, error) {
	records, warnings, err := readJSONL(sourceFS, "source/records.jsonl")
	if err != nil {
		return domain.Normalization{}, err
	}
	parentKeys := make(map[string]string)
	var events []domain.Event
	for _, record := range records {
		if err := ctx.Err(); err != nil {
			return domain.Normalization{}, err
		}
		value := record.Value
		message := object(value["message"])
		if message == nil {
			warnings = append(warnings, unknown(record, "Claude record has no supported message"))
			continue
		}
		id := stringValue(value["uuid"])
		parent := stringValue(value["parentUuid"])
		role := stringValue(message["role"])
		blocks, ok := message["content"].([]any)
		if !ok {
			blocks = []any{map[string]any{"type": "text", "text": contentText(message["content"])}}
		}
		firstEvent := len(events)
		for index, raw := range blocks {
			if err := ctx.Err(); err != nil {
				return domain.Normalization{}, err
			}
			block := object(raw)
			kind := stringValue(block["type"])
			event := domain.Event{
				Role: role, Model: stringValue(message["model"]), Timestamp: timestamp(value["timestamp"]),
				ParentKey: parent, Sources: source(record.Path, record.Line),
			}
			switch kind {
			case "text":
				event.Kind, event.Text = "message", stringValue(block["text"])
			case "thinking":
				event.Kind, event.Text = "reasoning", stringValue(block["thinking"])
			case "tool_use":
				event.Kind, event.Tool, event.CallID = "tool_call", stringValue(block["name"]), stringValue(block["id"])
				event.Text = readableJSON(block["input"])
			case "tool_result":
				event.Kind, event.CallID, event.Text = "tool_result", stringValue(block["tool_use_id"]), contentText(block["content"])
			case "image", "document":
				event.Kind = "attachment"
				event.Attachments = []domain.Attachment{{Name: stringValue(block["name"]), MediaType: stringValue(object(block["source"])["media_type"]), Path: stringValue(block["path"]), URL: stringValue(block["url"])}}
			default:
				warnings = append(warnings, unknown(record, "unknown Claude content "+kind))
				continue
			}
			event.Key = nativeKey(event.Kind, indexedID(id, index), map[string]any{"message": message, "block": block, "parent": parent})
			events = append(events, event)
		}
		if id != "" && len(events) > firstEvent {
			parentKeys[id] = events[firstEvent].Key
		}
	}
	// Resolve native parent UUIDs only after all records have produced their event keys.
	for index := range events {
		events[index].ParentKey = parentKeys[events[index].ParentKey]
	}
	return complete(events, warnings), nil
}

func normalizeCursorAgent(ctx context.Context, sourceFS fs.FS) (domain.Normalization, error) {
	records, warnings, err := readJSONL(sourceFS, "source/records.jsonl")
	if err != nil {
		return domain.Normalization{}, err
	}
	var events []domain.Event
	for _, record := range records {
		if err := ctx.Err(); err != nil {
			return domain.Normalization{}, err
		}
		value := record.Value
		role := stringValue(value["role"])
		message := object(value["message"])
		if role == "" {
			role = stringValue(message["role"])
		}
		if role != "user" && role != "assistant" && role != "system" {
			warnings = append(warnings, unknown(record, "unsupported Cursor Agent row"))
			continue
		}
		content := value["content"]
		if content == nil {
			content = message["content"]
		}
		id := stringValue(value["id"])
		if id == "" {
			id = stringValue(message["id"])
		}
		parent := stringValue(value["parentMessageId"])
		if parent == "" {
			parent = stringValue(message["parentMessageId"])
		}
		event := domain.Event{Kind: "message", Role: role, Model: stringValue(value["model"]), Timestamp: timestamp(value["timestamp"]), Text: contentText(content), Sources: source(record.Path, record.Line)}
		if parent != "" {
			event.ParentKey = nativeKey("message", parent, nil)
		}
		event.Key = nativeKey(event.Kind, id, map[string]any{"role": role, "content": content, "parent": parent})
		if blocks, ok := content.([]any); ok {
			for _, raw := range blocks {
				if err := ctx.Err(); err != nil {
					return domain.Normalization{}, err
				}
				block := object(raw)
				switch stringValue(block["type"]) {
				case "text":
				case "tool_use":
					tool := event
					tool.Kind, tool.Tool, tool.CallID = "tool_call", stringValue(block["name"]), stringValue(block["id"])
					tool.Text = readableJSON(block["input"])
					tool.Key = nativeKey(tool.Kind, tool.CallID, map[string]any{"record": value, "block": block})
					events = append(events, tool)
				case "tool_result":
					tool := event
					tool.Kind, tool.CallID, tool.Text = "tool_result", stringValue(block["tool_use_id"]), contentText(block["content"])
					tool.Key = nativeKey(tool.Kind, tool.CallID, map[string]any{"record": value, "block": block})
					events = append(events, tool)
				default:
					warnings = append(warnings, unknown(record, "unknown Cursor Agent content "+stringValue(block["type"])))
				}
			}
		}
		if event.Text != "" {
			events = append(events, event)
		}
	}
	return complete(events, warnings), nil
}
