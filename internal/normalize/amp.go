package normalize

import (
	"context"
	"encoding/json"
	"io/fs"

	"github.com/regutierrez/traicr/internal/domain"
)

func normalizeAmp(ctx context.Context, sourceFS fs.FS) (domain.Normalization, error) {
	export, err := readJSON(sourceFS, "source/export.json")
	if err != nil {
		return domain.Normalization{}, err
	}
	messages := objects(export["messages"])
	if len(messages) == 0 {
		thread := object(export["thread"])
		messages = objects(thread["messages"])
		if len(messages) == 0 {
			messages = objects(thread["events"])
		}
	}
	var events []domain.Event
	var warnings []domain.Warning
	for messageIndex, message := range messages {
		if err := ctx.Err(); err != nil {
			return domain.Normalization{}, err
		}
		id := firstString(message, "id", "messageId", "message_id")
		role := firstString(message, "role", "sender")
		model := firstString(message, "model", "modelId", "model_id")
		if model == "" {
			model = stringValue(object(message["usage"])["model"])
		}
		parent := firstString(message, "parentId", "parent_id")
		messageTimestamp := timestamp(message["timestamp"])
		if messageTimestamp == "" {
			messageTimestamp = timestamp(object(message["meta"])["sentAt"])
		}
		content := message["content"]
		blocks, isBlocks := content.([]any)
		if !isBlocks {
			blocks = []any{map[string]any{"type": "text", "text": contentText(content)}}
		}
		messageKey := nativeKey("amp-message", id, message)
		parentMessageKey := ""
		if parent != "" {
			parentMessageKey = nativeKey("amp-message", parent, nil)
		}
		for index, raw := range blocks {
			if err := ctx.Err(); err != nil {
				return domain.Normalization{}, err
			}
			block := object(raw)
			kind := firstString(block, "type", "kind")
			// Native exports can omit IDs and timestamps. Preserve their message
			// order separately from stable event identity for transcript rendering.
			metadata, _ := json.Marshal(map[string]any{"transcript_message": messageKey, "transcript_parent": parentMessageKey, "transcript_order": messageIndex, "transcript_block": index})
			event := domain.Event{Role: role, Model: model, Timestamp: messageTimestamp, Sources: source("source/export.json", 0), Metadata: metadata}
			if parent != "" {
				event.ParentKey = nativeKey("message", parent, nil)
			}
			switch kind {
			case "text", "message", "input_text", "output_text":
				event.Kind, event.Text = "message", contentText(block)
			case "thinking", "reasoning":
				event.Kind, event.Text = "reasoning", contentText(block)
			case "tool_use", "tool_call", "function_call":
				event.Kind = "tool_call"
				event.Tool = firstString(block, "name", "tool")
				// Amp results refer to its native tool-use ID, not the provider's ID.
				event.CallID = firstString(block, "toolUseID", "id", "providerToolUseId", "callId", "call_id")
				event.Text = readableJSON(firstValue(block, "input", "arguments"))
			case "tool_result", "function_call_output":
				event.Kind = "tool_result"
				event.CallID = firstString(block, "toolUseID", "providerToolUseId", "tool_use_id", "callId", "call_id")
				event.Text = ampResultText(block)
			case "image", "attachment":
				event.Kind = "attachment"
				event.Attachments = []domain.Attachment{{Name: firstString(block, "name", "filename"), MediaType: firstString(block, "mimeType", "media_type"), Path: stringValue(block["path"]), URL: stringValue(block["url"])}}
			default:
				warnings = append(warnings, warning("unsupported_record", "source/export.json: unknown Amp content "+kind))
				continue
			}
			blockID := stringValue(block["id"])
			if blockID == "" {
				blockID = indexedID(id, index)
			}
			event.Key = nativeKey(event.Kind, blockID, map[string]any{"message": message, "block": block})
			events = append(events, event)
		}
	}
	if len(messages) == 0 {
		warnings = append(warnings, warning("unsupported_version", "Amp export has no supported messages"))
	}
	return complete(events, warnings), nil
}

func ampResultText(block map[string]any) string {
	if text := contentText(firstValue(block, "content", "output", "result")); text != "" {
		return text
	}
	result := object(object(block["run"])["result"])
	for _, key := range []string{"output", "message", "summary"} {
		if text := contentText(result[key]); text != "" {
			return text
		}
	}
	return ""
}

func firstString(value map[string]any, keys ...string) string {
	for _, key := range keys {
		if found := stringValue(value[key]); found != "" {
			return found
		}
	}
	return ""
}

func firstValue(value map[string]any, keys ...string) any {
	for _, key := range keys {
		if value[key] != nil {
			return value[key]
		}
	}
	return nil
}
