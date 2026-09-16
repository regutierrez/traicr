package normalize

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"

	"github.com/regutierrez/traicr/internal/amp"
	"github.com/regutierrez/traicr/internal/domain"
)

func normalizeAmp(ctx context.Context, sourceFS fs.FS, files []domain.File) (domain.Normalization, error) {
	export, err := readJSON(sourceFS, "source/export.json")
	if err != nil {
		return domain.Normalization{}, err
	}
	archived := make(map[string]int64, len(files))
	for _, file := range files {
		archived[file.Path] = file.Size
	}
	messages := objects(export["messages"])
	messagePath := "/messages"
	if len(messages) == 0 {
		thread := object(export["thread"])
		messages = objects(thread["messages"])
		messagePath = "/thread/messages"
		if len(messages) == 0 {
			messages = objects(thread["events"])
			messagePath = "/thread/events"
		}
	}
	var events []domain.Event
	var warnings []domain.Warning
	missingImages := make(map[string]bool)
	if id := stringValue(export["id"]); id != "" {
		session := make(map[string]any)
		for _, key := range []string{"id", "title", "created", "updatedAt", "archived", "agentMode", "activatedSkills", "env", "meta"} {
			if value := export[key]; value != nil {
				session[key] = searchableValue("", value)
			}
		}
		metadata, _ := json.Marshal(map[string]any{"session": session, "transcript_order": -1, "transcript_message": "amp-session:" + id})
		events = append(events, domain.Event{Key: "session_info:" + id, Kind: "session_info", Timestamp: timestamp(export["created"]), Text: contentText(session), Metadata: metadata, Sources: source("source/export.json", 0)})
	}
	tools := make(map[string]string)
	for _, message := range messages {
		for _, block := range objects(message["content"]) {
			if firstString(block, "type", "kind") == "tool_use" {
				tools[firstString(block, "toolUseID", "id", "providerToolUseId")] = firstString(block, "name", "tool")
			}
		}
	}
	for messageIndex, message := range messages {
		if err := ctx.Err(); err != nil {
			return domain.Normalization{}, err
		}
		id := numberString(firstValue(message, "id", "messageId", "message_id"))
		role := firstString(message, "role", "sender")
		model := firstString(message, "model", "modelId", "model_id")
		if model == "" {
			model = stringValue(object(message["usage"])["model"])
		}
		parent := numberString(firstValue(message, "parentId", "parent_id"))
		messageTimestamp := timestamp(firstValue(message, "timestamp", "createdAt"))
		if messageTimestamp == "" {
			messageTimestamp = timestamp(object(message["meta"])["sentAt"])
		}
		content := message["content"]
		blocks, isBlocks := content.([]any)
		contentPointer := fmt.Sprintf("%s/%d/content", messagePath, messageIndex)
		if !isBlocks {
			blocks = []any{map[string]any{"type": "text", "text": contentText(content)}}
		}
		if len(blocks) == 0 && message["state"] != nil {
			blocks = []any{map[string]any{"type": "text", "text": ""}}
			isBlocks = false
			contentPointer = fmt.Sprintf("%s/%d", messagePath, messageIndex)
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
			pointer := contentPointer
			if isBlocks {
				pointer += fmt.Sprintf("/%d", index)
			}
			metadata := map[string]any{
				"transcript_message": messageKey, "transcript_parent": parentMessageKey, "transcript_order": messageIndex, "transcript_block": index,
				"source_pointer": pointer,
			}
			for key, value := range map[string]any{
				"run": block["run"], "usage": message["usage"], "message_meta": message["meta"], "state": message["state"],
				"hidden": block["hidden"], "block_state": block["blockState"], "complete": block["complete"],
			} {
				if value != nil {
					metadata[key] = searchableValue("", value)
				}
			}
			for key, field := range map[string]string{"start_time": "startTime", "final_time": "finalTime"} {
				if value := timestamp(block[field]); value != "" {
					metadata[key] = value
				}
			}
			event := domain.Event{Role: role, Model: model, Provider: firstString(block, "provider"), Timestamp: messageTimestamp, Sources: source("source/export.json", 0)}
			if event.Timestamp == "" {
				event.Timestamp = timestamp(firstValue(block, "startTime", "finalTime"))
				if event.Timestamp == "" {
					event.Timestamp = timestamp(object(message["usage"])["timestamp"])
				}
			}
			if parent != "" {
				event.ParentKey = nativeKey("message", parent, nil)
			}
			switch kind {
			case "text", "message", "input_text", "output_text":
				event.Kind, event.Text = "message", contentText(block)
			case "thinking", "reasoning":
				event.Kind, event.Text = "reasoning", contentText(block)
			case "summary":
				event.Kind, event.Text = "compaction", contentText(block["summary"])
			case "tool_use", "tool_call", "function_call":
				event.Kind = "tool_call"
				event.Tool = firstString(block, "name", "tool")
				// Amp results refer to its native tool-use ID, not the provider's ID.
				event.CallID = firstString(block, "toolUseID", "id", "providerToolUseId", "callId", "call_id")
				event.Text = readableJSON(firstValue(block, "input", "arguments"))
			case "tool_result", "function_call_output":
				event.Kind = "tool_result"
				event.CallID = firstString(block, "toolUseID", "providerToolUseId", "tool_use_id", "callId", "call_id")
				event.Tool = tools[event.CallID]
				event.Text = ampResultText(block)
				event.Attachments = amp.Attachments(block, pointer)
			case "image", "attachment":
				event.Kind = "attachment"
				event.Attachments = amp.Attachments(block, pointer)
			default:
				warnings = append(warnings, warning("unsupported_record", "source/export.json: unknown Amp content "+kind))
				event.Kind, event.Text = "unknown", readableJSON(raw)
				metadata["native_type"] = kind
			}
			for i := range event.Attachments {
				attachment := &event.Attachments[i]
				_, path := amp.AttachmentLocation(attachment.URL)
				if size, ok := archived[path]; ok && path != "" {
					attachment.ArchivedPath, attachment.Size = path, size
				} else if path != "" && !attachment.Inline && !missingImages[path] {
					missingImages[path] = true
					warnings = append(warnings, warning("amp_image_unavailable", "Hosted image at "+attachment.SourcePointer+" is not archived. Recollect and upload with an authenticated Amp CLI to retry."))
				}
			}
			blockID := stringValue(block["id"])
			if blockID == "" {
				blockID = indexedID(id, index)
			}
			event.Key = nativeKey(event.Kind, blockID, map[string]any{"message": message, "block": block})
			legacyID := stringValue(block["id"])
			if legacyID == "" {
				legacyID = indexedID(firstString(message, "id", "messageId", "message_id"), index)
			}
			legacyKey := nativeKey(event.Kind, legacyID, map[string]any{"message": message, "block": block})
			if legacyKey != event.Key {
				event.LegacyKeys = []string{legacyKey}
			}
			event.Metadata, _ = json.Marshal(metadata)
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
	run := object(block["run"])
	result := firstValue(run, "error", "result")
	if text := contentText(result); text != "" {
		return text
	}
	if message := stringValue(object(result)["message"]); message != "" {
		return message
	}
	return readableJSON(result)
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
