package normalize

import (
	"context"
	"encoding/json"
	"io/fs"
	"strings"

	"github.com/regutierrez/traicr/internal/domain"
)

// Claude Code writes session bookkeeping next to the conversation. These records
// carry nothing to show; they still take part in the parent chain.
var claudeMetadataRecords = map[string]bool{
	"ai-title": true, "custom-title": true, "last-prompt": true, "atis-latch": true, "isolation-latch": true,
	"mode": true, "permission-mode": true, "api-request": true, "api-request-shape": true,
	"queue-operation": true, "file-history-snapshot": true, "file-history-delta": true, "cost-state": true,
	"pr-link": true, "agent-name": true, "continued-in": true, "attachment": true,
}

// System records that are timing or hook bookkeeping rather than something said.
var claudeQuietSystemRecords = map[string]bool{"turn_duration": true, "stop_hook_summary": true, "agents_killed": true}

func normalizeClaude(ctx context.Context, sourceFS fs.FS) (domain.Normalization, error) {
	records, warnings, err := readJSONL(sourceFS, "source/records.jsonl")
	if err != nil {
		return domain.Normalization{}, err
	}
	// Records point at their parent by UUID, often through records that produce no
	// events (attachments, hooks). Resolve through those so every event keeps its place.
	parents := make(map[string]string)
	for _, record := range records {
		if id := stringValue(record.Value["uuid"]); id != "" {
			parents[id] = claudeParent(record.Value)
		}
	}
	keys := make(map[string]string)
	// An event without its own UUID (a system prompt) is placed after the current
	// head of the conversation; later records that point at that head follow it.
	inserted := make(map[string]string)
	resolve := func(id string) string {
		for seen := 0; id != "" && seen < len(parents)+1; seen++ {
			if key, ok := keys[id]; ok {
				for hops := 0; hops <= len(inserted); hops++ {
					next, ok := inserted[key]
					if !ok {
						break
					}
					key = next
				}
				return key
			}
			id = parents[id]
		}
		return ""
	}
	var events []domain.Event
	parentOfKey := make(map[string]string)
	emit := func(event domain.Event) {
		events = append(events, event)
		parentOfKey[event.Key] = event.ParentKey
	}
	// isAncestor reports whether ancestor is on the chain above key, looking a bounded way up.
	isAncestor := func(ancestor, key string) bool {
		for steps := 0; key != "" && steps < 1000; steps++ {
			if key == ancestor {
				return true
			}
			key = parentOfKey[key]
		}
		return false
	}
	var title, customTitle, effort, head string
	prompts := make(map[string]bool)
	// Parents written after their children are resolved once every record is read.
	late := make(map[int]string)
	for _, record := range records {
		if err := ctx.Err(); err != nil {
			return domain.Normalization{}, err
		}
		value := record.Value
		kind := stringValue(value["type"])
		id := stringValue(value["uuid"])
		previous := head
		if id != "" {
			head = id
		}
		base := domain.Event{Timestamp: timestamp(value["timestamp"]), ParentKey: resolve(claudeParent(value)), Sources: source(record.Path, record.Line)}
		switch {
		case kind == "ai-title":
			title = strings.TrimSpace(stringValue(value["aiTitle"]))
			continue
		case kind == "custom-title":
			customTitle = strings.TrimSpace(stringValue(value["customTitle"]))
			continue
		case kind == "attachment" && stringValue(object(value["attachment"])["type"]) == "queued_command":
			// A message typed while the agent was working, or a background task report.
			attachment := object(value["attachment"])
			event := base
			event.Kind, event.Role, event.Text = "message", "system", contentText(attachment["prompt"])
			if stringValue(attachment["commandMode"]) == "prompt" {
				event.Role = "user"
			}
			event.Key = nativeKey(event.Kind, indexedID(id, 0), value)
			emit(event)
			keys[id] = event.Key
			continue
		case kind == "api-request-blob":
			// The request sent to the model. Its user and assistant messages repeat the
			// conversation; only the system prompt is new, and it is shown once per version.
			message := object(value["message"])
			hash := stringValue(value["hash"])
			if stringValue(message["role"]) != "system" || prompts[hash] {
				continue
			}
			prompts[hash] = true
			event := base
			event.Kind, event.Role, event.Text = "message", "system", contentText(message["content"])
			event.ParentKey = resolve(head)
			event.Key = nativeKey(event.Kind, "prompt:"+hash, message)
			emit(event)
			if event.ParentKey != "" {
				inserted[event.ParentKey] = event.Key
			}
			continue
		case kind == "system":
			subtype := stringValue(value["subtype"])
			event := base
			event.Kind, event.Role = "message", "system"
			switch {
			case claudeQuietSystemRecords[subtype]:
				continue
			case subtype == "compact_boundary":
				event.Kind, event.Role, event.Text = "compaction", "", stringValue(value["content"])
			case subtype == "api_error":
				failure := object(value["error"])
				event.Text = "API error: " + firstString(failure, "formatted", "message")
			case value["content"] != nil:
				event.Text = contentText(value["content"])
			default:
				warnings = append(warnings, unknown(record, "unknown Claude system record "+subtype))
				continue
			}
			event.Key = nativeKey(event.Kind, indexedID(id, 0), value)
			emit(event)
			keys[id] = event.Key
			continue
		case claudeMetadataRecords[kind]:
			continue
		}
		message := object(value["message"])
		if message == nil || kind != "" && kind != "user" && kind != "assistant" {
			warnings = append(warnings, unknown(record, "unknown Claude record "+kind))
			continue
		}
		role := stringValue(message["role"])
		blocks, ok := message["content"].([]any)
		if !ok {
			blocks = []any{map[string]any{"type": "text", "text": contentText(message["content"])}}
		}
		following := ""
		if role == "user" && claudeToolResults(blocks) && base.ParentKey != "" {
			// Parallel tool calls each point their result at their own call, which would
			// make every result but the last a side branch. Results follow the conversation
			// in the order they were written; later records pointing at an earlier one follow too.
			if tail := resolve(previous); tail != base.ParentKey && isAncestor(base.ParentKey, tail) {
				base.ParentKey = tail
				following = tail
			}
		}
		if role == "user" && claudeInjected(value, blocks) {
			// Skill bodies, reminders and task reports reach the model as user messages
			// but nobody typed them.
			role = "system"
		}
		if level := stringValue(value["effort"]); role == "assistant" && level != "" && level != effort {
			effort = level
			change := base
			change.Kind, change.Text = "thinking_level_change", level
			change.Key = nativeKey(change.Kind, "effort:"+id, nil)
			emit(change)
			base.ParentKey = change.Key
		}
		firstEvent := len(events)
		for index, raw := range blocks {
			if err := ctx.Err(); err != nil {
				return domain.Normalization{}, err
			}
			block := object(raw)
			event := base
			event.Role, event.Model = role, stringValue(message["model"])
			switch stringValue(block["type"]) {
			case "text":
				event.Kind, event.Text = "message", stringValue(block["text"])
				if value["isCompactSummary"] == true {
					event.Kind, event.Role = "compaction", ""
					event.LegacyKeys = []string{nativeKey("message", indexedID(id, index), map[string]any{"message": message, "block": block, "parent": stringValue(value["parentUuid"])})}
				}
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
				warnings = append(warnings, unknown(record, "unknown Claude content "+stringValue(block["type"])))
				continue
			}
			event.Key = nativeKey(event.Kind, indexedID(id, index), map[string]any{"message": message, "block": block, "parent": stringValue(value["parentUuid"])})
			if event.ParentKey == "" && claudeParent(value) != "" {
				late[len(events)] = claudeParent(value)
			}
			emit(event)
		}
		if id != "" && len(events) > firstEvent {
			keys[id] = events[firstEvent].Key
			if following != "" {
				inserted[following] = events[firstEvent].Key
			}
		}
	}
	for index, parent := range late {
		events[index].ParentKey = resolve(parent)
	}
	result := complete(events, warnings)
	if customTitle != "" {
		title = customTitle
	}
	result.Title = title
	return result, nil
}

func claudeParent(value map[string]any) string {
	if parent := stringValue(value["parentUuid"]); parent != "" {
		return parent
	}
	// After a compaction the chain restarts; logicalParentUuid keeps the link.
	return stringValue(value["logicalParentUuid"])
}

func claudeToolResults(blocks []any) bool {
	for _, raw := range blocks {
		if stringValue(object(raw)["type"]) != "tool_result" {
			return false
		}
	}
	return len(blocks) > 0
}

func claudeInjected(value map[string]any, blocks []any) bool {
	if value["isMeta"] == true {
		return true
	}
	for _, raw := range blocks {
		text := strings.TrimSpace(stringValue(object(raw)["text"]))
		if strings.HasPrefix(text, "<task-notification>") {
			return true
		}
	}
	return false
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
		if stringValue(value["type"]) == "turn_ended" {
			// Only a turn that failed or was stopped has something to show.
			if status := stringValue(value["status"]); status != "success" {
				state := map[string]any{"stopReason": status, "errorMessage": stringValue(value["error"])}
				event := domain.Event{Kind: "message", Role: "assistant", Sources: source(record.Path, record.Line)}
				event.Metadata, _ = json.Marshal(map[string]any{"state": state})
				previous := ""
				if len(events) > 0 {
					previous = events[len(events)-1].Key
				}
				event.Key = nativeKey(event.Kind, "", map[string]any{"after": previous, "record": value})
				events = append(events, event)
			}
			continue
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
	// Cursor sometimes writes a row again unchanged, such as the same prompt after a
	// context injection; chainInFileOrder keeps that copy from starting a branch.
	chainInFileOrder(events)
	return complete(events, warnings), nil
}
