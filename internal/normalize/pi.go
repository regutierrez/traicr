package normalize

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"slices"
	"sort"
	"strings"

	"github.com/regutierrez/traicr/internal/domain"
)

type piEntry struct {
	record   sourceRecord
	id       string
	parentID string
	model    string
	provider string
	events   []domain.Event
}

type piModelContext struct {
	model    string
	provider string
}

func normalizePi(ctx context.Context, sourceFS fs.FS) (domain.Normalization, error) {
	records, warnings, err := readJSONL(sourceFS, "source/records.jsonl")
	if err != nil {
		return domain.Normalization{}, err
	}
	if len(records) == 0 {
		return complete(nil, warnings), nil
	}
	header := records[0].Value
	if stringValue(header["type"]) != "session" || numberString(header["version"]) != "3" {
		warnings = append(warnings, warning("unsupported_version", "Pi session header is missing or is not version 3"))
		return complete(nil, warnings), nil
	}

	entries, graphWarnings, err := orderPiEntries(ctx, records[1:])
	warnings = append(warnings, graphWarnings...)
	if err != nil {
		return domain.Normalization{}, err
	}
	primaryKeys := make(map[string]string, len(entries))
	for _, entry := range entries {
		entryEvents, entryWarnings, err := decodePiRecord(ctx, entry)
		warnings = append(warnings, entryWarnings...)
		if err != nil {
			return domain.Normalization{}, err
		}
		entry.events = entryEvents
		if len(entryEvents) > 0 {
			primaryKeys[entry.id] = entryEvents[0].Key
		}
	}

	var events []domain.Event
	for _, entry := range entries {
		for _, event := range entry.events {
			event.ParentKey = primaryKeys[entry.parentID]
			events = append(events, event)
		}
	}
	return complete(events, warnings), nil
}

// Pi stores entries as a parent graph, so file order cannot provide either event order or model inheritance.
func orderPiEntries(ctx context.Context, records []sourceRecord) ([]*piEntry, []domain.Warning, error) {
	entries := make(map[string]*piEntry, len(records))
	var warnings []domain.Warning
	for _, record := range records {
		if err := ctx.Err(); err != nil {
			return nil, warnings, err
		}
		id := stringValue(record.Value["id"])
		if id == "" {
			warnings = append(warnings, unknown(record, "entry has no native id"))
			continue
		}
		if previous, exists := entries[id]; exists {
			warnings = append(warnings, warning("duplicate_native_id", fmt.Sprintf("%s line %d duplicates Pi entry %s from line %d", record.Path, record.Line, id, previous.record.Line)))
			continue
		}
		entries[id] = &piEntry{record: record, id: id, parentID: stringValue(record.Value["parentId"])}
	}

	ids := make([]string, 0, len(entries))
	for id := range entries {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	state := make(map[string]uint8, len(entries))
	ordered := make([]*piEntry, 0, len(entries))
	var visitErr error
	var visit func(string)
	visit = func(id string) {
		if visitErr != nil || state[id] == 2 {
			return
		}
		if err := ctx.Err(); err != nil {
			visitErr = err
			return
		}
		if state[id] == 1 {
			warnings = append(warnings, warning("invalid_parent_graph", fmt.Sprintf("Pi entry %s is in a parent cycle", id)))
			return
		}
		state[id] = 1
		entry := entries[id]
		if entry.parentID != "" {
			if entries[entry.parentID] != nil {
				visit(entry.parentID)
			} else {
				warnings = append(warnings, warning("missing_parent", fmt.Sprintf("Pi entry %s references missing parent %s", id, entry.parentID)))
			}
		}
		state[id] = 2
		ordered = append(ordered, entry)
	}
	for _, id := range ids {
		visit(id)
	}
	if visitErr != nil {
		return nil, warnings, visitErr
	}

	contexts := make(map[string]piModelContext, len(entries))
	for _, entry := range ordered {
		model := contexts[entry.parentID]
		if stringValue(entry.record.Value["type"]) == "model_change" {
			model = piModelContext{model: stringValue(entry.record.Value["modelId"]), provider: stringValue(entry.record.Value["provider"])}
		}
		entry.model, entry.provider = model.model, model.provider
		contexts[entry.id] = model
	}
	return ordered, warnings, nil
}

func decodePiRecord(ctx context.Context, entry *piEntry) ([]domain.Event, []domain.Warning, error) {
	value := entry.record.Value
	base := domain.Event{Model: entry.model, Provider: entry.provider, Timestamp: timestamp(value["timestamp"]), Sources: source(entry.record.Path, entry.record.Line)}
	kind := stringValue(value["type"])
	if kind != "message" {
		event := base
		event.Kind = kind
		event.Key = nativeKey(kind, entry.id, value)
		switch kind {
		case "model_change":
			event.Text = event.Provider + "/" + event.Model
		case "compaction", "branch_summary":
			event.Text = stringValue(value["summary"])
			if kind == "branch_summary" {
				event.Branch = stringValue(value["fromId"])
			}
		case "thinking_level_change", "custom_message", "session_info", "label":
			event.Text = contentText(value)
		case "custom":
			event.Text = stringValue(value["customType"])
		case "context_edit":
			// Changes only what the model sees next; the target entry stays in history. Null replacement omits it.
			event.Text = contentText(value["replacement"])
		default:
			return nil, []domain.Warning{unknown(entry.record, "unknown Pi entry type "+kind)}, nil
		}
		return []domain.Event{event}, nil, nil
	}

	message := object(value["message"])
	role := stringValue(message["role"])
	if explicit := stringValue(message["model"]); explicit != "" {
		base.Model = explicit
	}
	if explicit := stringValue(message["provider"]); explicit != "" {
		base.Provider = explicit
	}
	if role == "bashExecution" {
		return piBashExecution(base, entry.id, value, message), nil, nil
	}
	content, structured := message["content"].([]any)
	if !structured {
		if role == "system" && contentText(message["content"]) == "" {
			event := base
			event.Kind, event.Role, event.Text = "message", role, piSystemPrompt(message)
			event.Key = nativeKey(event.Kind, entry.id, value)
			return []domain.Event{event}, nil, nil
		}
		event := base
		event.Kind, event.Role, event.Text = "message", role, contentText(message["content"])
		event.CallID, event.Tool = stringValue(message["toolCallId"]), stringValue(message["toolName"])
		if role == "toolResult" || event.CallID != "" {
			event.Kind = "tool_result"
		}
		event.Key = nativeKey(event.Kind, entry.id, value)
		return []domain.Event{event}, nil, nil
	}

	// Failed and aborted replies often have no content; keep them so the page can show what happened.
	stopReason, errorMessage := stringValue(message["stopReason"]), stringValue(message["errorMessage"])
	if role == "assistant" && (errorMessage != "" || stopReason != "" && stopReason != "stop" && stopReason != "toolUse") {
		if stopReason == "" {
			stopReason = "error"
		}
		state := map[string]any{"stopReason": stopReason}
		if errorMessage != "" {
			state["errorMessage"] = errorMessage
		}
		base.Metadata, _ = json.Marshal(map[string]any{"state": state})
		if len(content) == 0 {
			content = []any{map[string]any{"type": "text", "text": ""}}
		}
	}

	var events []domain.Event
	var warnings []domain.Warning
	for index, raw := range content {
		if err := ctx.Err(); err != nil {
			return nil, warnings, err
		}
		block := object(raw)
		event := base
		event.Role = role
		if role == "toolResult" {
			event.Kind, event.Text = "tool_result", contentText(block)
			event.CallID, event.Tool = stringValue(message["toolCallId"]), stringValue(message["toolName"])
		} else {
			switch stringValue(block["type"]) {
			case "text":
				event.Kind, event.Text = "message", stringValue(block["text"])
			case "thinking":
				event.Kind, event.Text = "reasoning", stringValue(block["thinking"])
			case "toolCall":
				event.Kind, event.Tool, event.CallID = "tool_call", stringValue(block["name"]), stringValue(block["id"])
				event.Text = readableJSON(block["arguments"])
			case "image":
				event.Kind = "attachment"
				event.Attachments = []domain.Attachment{{Name: stringValue(block["name"]), MediaType: firstString(block, "mimeType", "mediaType"), Path: stringValue(block["path"]), URL: stringValue(block["url"])}}
			default:
				warnings = append(warnings, unknown(entry.record, "unknown Pi message content "+stringValue(block["type"])))
				continue
			}
		}
		event.Key = nativeKey(event.Kind, indexedID(entry.id, index), block)
		events = append(events, event)
	}
	return events, warnings, nil
}

// A user-run "!command" stores the command and its output on one message, so it becomes a call and its result.
func piBashExecution(base domain.Event, id string, value, message map[string]any) []domain.Event {
	call := base
	call.Kind, call.Role, call.Tool, call.CallID = "tool_call", "user", "bash", "bash:"+id
	call.Text = readableJSON(map[string]any{"command": stringValue(message["command"])})
	call.Key = nativeKey(call.Kind, indexedID(id, 0), value)
	call.LegacyKeys = []string{nativeKey("message", id, value)}

	result := base
	result.Kind, result.Role, result.Tool, result.CallID = "tool_result", "user", "bash", call.CallID
	result.Text = stringValue(message["output"])
	result.Key = nativeKey(result.Kind, indexedID(id, 1), value)
	run := map[string]any{"status": "done"}
	if code, ok := wholeNumber(message["exitCode"]); ok {
		run["result"] = map[string]any{"exitCode": code}
		if code != 0 {
			run["status"] = "error"
		}
	}
	if cancelled, _ := message["cancelled"].(bool); cancelled {
		run["status"] = "cancelled"
	}
	result.Metadata, _ = json.Marshal(map[string]any{"run": run})
	return []domain.Event{call, result}
}

// Pi bridges that build their own system prompt store it as named sections and leave content empty.
// Later system records are updates: they carry only the sections that changed, "" for a cleared
// section, and the tools added or removed.
var piSystemSectionOrder = []string{"preamble", "tools", "rules", "docs", "addendum", "skills", "cwd", "mcp_servers"}

func piSystemPrompt(message map[string]any) string {
	sections := object(message["sections"])
	names := make([]string, 0, len(sections))
	for name := range sections {
		if !slices.Contains(piSystemSectionOrder, name) {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	var parts, cleared []string
	for _, name := range append(slices.Clone(piSystemSectionOrder), names...) {
		if _, present := sections[name]; !present {
			continue
		}
		if text := strings.TrimSpace(stringValue(sections[name])); text != "" {
			parts = append(parts, text)
		} else {
			cleared = append(cleared, name)
		}
	}
	if len(cleared) > 0 {
		parts = append(parts, "Cleared sections: "+strings.Join(cleared, ", "))
	}
	for _, change := range []struct{ field, label string }{{"toolsAdded", "Tools added: "}, {"toolsRemoved", "Tools removed: "}} {
		var tools []string
		for _, tool := range objects(message[change.field]) {
			if name := stringValue(tool["name"]); name != "" {
				tools = append(tools, name)
			}
		}
		if len(tools) > 0 {
			parts = append(parts, change.label+strings.Join(tools, ", "))
		}
	}
	return strings.Join(parts, "\n\n")
}
