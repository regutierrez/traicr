package normalize

import (
	"context"
	"io/fs"

	"github.com/regutierrez/traicr/internal/domain"
)

func normalizeOpenCode(ctx context.Context, sourceFS fs.FS) (domain.Normalization, error) {
	export, err := readJSON(sourceFS, "source/export.json")
	if err != nil {
		return domain.Normalization{}, err
	}
	var events []domain.Event
	var warnings []domain.Warning
	messages := objects(export["messages"])
	parentKeys := make(map[string]string)
	for _, envelope := range messages {
		if err := ctx.Err(); err != nil {
			return domain.Normalization{}, err
		}
		info := object(envelope["info"])
		parts := objects(envelope["parts"])
		if len(parts) > 0 {
			parentKeys[stringValue(info["id"])] = nativeKey(openCodePartKind(parts[0]), stringValue(parts[0]["id"]), parts[0])
		}
	}
	for _, envelope := range messages {
		if err := ctx.Err(); err != nil {
			return domain.Normalization{}, err
		}
		info := object(envelope["info"])
		messageID := stringValue(info["id"])
		role := stringValue(info["role"])
		if role == "" {
			role = stringValue(info["type"])
		}
		model, provider := openCodeModel(info)
		for _, part := range objects(envelope["parts"]) {
			if err := ctx.Err(); err != nil {
				return domain.Normalization{}, err
			}
			kind := stringValue(part["type"])
			id := stringValue(part["id"])
			event := domain.Event{Role: role, Model: model, Provider: provider, ParentKey: parentKeys[stringValue(info["parentID"])], Timestamp: timestamp(object(info["time"])["created"]), Sources: source("source/export.json", 0)}
			switch kind {
			case "text":
				event.Kind, event.Text = "message", stringValue(part["text"])
			case "reasoning":
				event.Kind, event.Text = "reasoning", stringValue(part["text"])
			case "tool":
				event.Tool = stringValue(part["tool"])
				if event.Tool == "" {
					event.Tool = stringValue(part["name"])
				}
				event.CallID = stringValue(part["callID"])
				if event.CallID == "" {
					event.CallID = stringValue(part["id"])
				}
				state := object(part["state"])
				event.Kind, event.Text = "tool_call", readableJSON(state["input"])
				event.Key = nativeKey(event.Kind, id, map[string]any{"messageID": messageID, "part": part})
				events = append(events, event)
				if state["output"] != nil || state["result"] != nil || state["error"] != nil {
					event.Kind, event.Text = "tool_result", contentText(state)
					event.Key = nativeKey(event.Kind, id, map[string]any{"messageID": messageID, "part": part})
					events = append(events, event)
				}
				continue
			case "file":
				event.Kind = "attachment"
				event.Attachments = []domain.Attachment{{Name: stringValue(part["filename"]), MediaType: stringValue(part["mime"]), URL: stringValue(part["url"])}}
			case "compaction", "patch", "snapshot", "step-finish", "subtask":
				event.Kind, event.Text = kind, contentText(part)
			default:
				warnings = append(warnings, warning("unsupported_record", "source/export.json: unknown OpenCode part "+kind))
				continue
			}
			event.Key = nativeKey(event.Kind, id, map[string]any{"messageID": messageID, "part": part})
			events = append(events, event)
		}
	}
	if len(messages) == 0 {
		warnings = append(warnings, warning("unsupported_version", "OpenCode export has no supported messages envelope"))
	}
	return complete(events, warnings), nil
}

func openCodePartKind(part map[string]any) string {
	switch stringValue(part["type"]) {
	case "text":
		return "message"
	case "tool":
		return "tool_call"
	default:
		return stringValue(part["type"])
	}
}

func openCodeModel(info map[string]any) (string, string) {
	model := object(info["model"])
	if model != nil {
		return stringValue(model["modelID"]), stringValue(model["providerID"])
	}
	return stringValue(info["modelID"]), stringValue(info["providerID"])
}
