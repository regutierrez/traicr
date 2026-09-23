package normalize

import (
	"bytes"
	"context"
	"encoding/json"
	"io/fs"
	"strings"

	"github.com/regutierrez/traicr/internal/domain"
)

type cursorRow struct {
	Record sourceRecord
	Table  string
	Key    string
	Value  map[string]any
}

func normalizeCursor(ctx context.Context, sourceFS fs.FS) (domain.Normalization, error) {
	records, warnings, err := readJSONL(sourceFS, "source/rows.jsonl")
	if err != nil {
		return domain.Normalization{}, err
	}
	var rows []cursorRow
	for _, record := range records {
		if err := ctx.Err(); err != nil {
			return domain.Normalization{}, err
		}
		value := record.Value
		var decoded map[string]any
		if err := json.Unmarshal([]byte(stringValue(value["value"])), &decoded); err != nil {
			warnings = append(warnings, unknown(record, "Cursor row value is not a JSON object"))
			continue
		}
		rows = append(rows, cursorRow{Record: record, Table: stringValue(value["table"]), Key: stringValue(value["key"]), Value: decoded})
	}

	byKey := make(map[string]cursorRow)
	var events []domain.Event
	var composers []cursorRow
	for _, row := range rows {
		if err := ctx.Err(); err != nil {
			return domain.Normalization{}, err
		}
		byKey[row.Key] = row
		switch {
		case row.Table == "cursorDiskKV" && strings.HasPrefix(row.Key, "composerData:"):
			composers = append(composers, row)
		case row.Table == "ItemTable" && (row.Key == "composer.composerData" || strings.Contains(row.Key, "aichat.chatdata")):
			parsed, partial := normalizeLegacyCursor(row)
			events = append(events, parsed...)
			warnings = append(warnings, partial...)
		case (row.Table == "cursorDiskKV" || row.Table == "ItemTable") && strings.HasPrefix(row.Key, "bubbleId:"):
		default:
			warnings = append(warnings, unknown(row.Record, "unsupported Cursor table or key"))
		}
	}
	for _, composer := range composers {
		if err := ctx.Err(); err != nil {
			return domain.Normalization{}, err
		}
		for _, header := range objects(composer.Value["fullConversationHeadersOnly"]) {
			if err := ctx.Err(); err != nil {
				return domain.Normalization{}, err
			}
			bubbleID := stringValue(header["bubbleId"])
			row, ok := byKey["bubbleId:"+stringValue(composer.Value["composerId"])+":"+bubbleID]
			if !ok {
				warnings = append(warnings, warning("missing_bubble", "Cursor composer references unavailable bubble "+bubbleID))
				continue
			}
			events = append(events, cursorBubble(row, header)...)
		}
	}
	return complete(events, warnings), nil
}

func cursorBubble(row cursorRow, header map[string]any) []domain.Event {
	id := stringValue(row.Value["bubbleId"])
	if id == "" {
		id = stringValue(header["bubbleId"])
	}
	role := ""
	switch numberString(row.Value["type"]) {
	case "1":
		role = "user"
	case "2":
		role = "assistant"
	}
	base := domain.Event{Role: role, Model: stringValue(row.Value["model"]), Timestamp: timestamp(row.Value["createdAt"]), Sources: source(row.Record.Path, row.Record.Line)}
	var events []domain.Event
	if text := stringValue(row.Value["text"]); text != "" {
		message := base
		message.Key, message.Kind, message.Text = nativeKey("message", id, row.Value), "message", text
		events = append(events, message)
	}
	if tool := object(row.Value["toolFormerData"]); tool != nil {
		call := base
		call.Kind, call.Tool = "tool_call", stringValue(tool["name"])
		// Composer bubbles store the provider id on toolCallId.
		call.CallID = firstString(tool, "toolCallId", "tool_call_id", "callId", "call_id", "id")
		if call.CallID == "" {
			call.CallID = id
		}
		call.Text = readableJSON(cursorToolArgs(tool))
		call.Key = nativeKey(call.Kind, id, row.Value)
		events = append(events, call)
		if tool["result"] != nil {
			result := call
			decoded := decodeCursorValue(tool["result"])
			result.Kind, result.Text = "tool_result", formatCursorResult(decoded, 6)
			result.Key = nativeKey(result.Kind, id, row.Value)
			if meta := cursorRunMetadata(tool, decoded); meta != nil {
				result.Metadata, _ = json.Marshal(meta)
			}
			events = append(events, result)
		}
	}
	return events
}

func cursorToolArgs(tool map[string]any) any {
	for _, key := range []string{"params", "rawArgs"} {
		value, ok := tool[key]
		if !ok || value == nil {
			continue
		}
		text, isString := value.(string)
		if !isString {
			return value
		}
		text = strings.TrimSpace(text)
		if text == "" {
			continue
		}
		var decoded any
		if json.Unmarshal([]byte(text), &decoded) == nil && decoded != nil {
			return decoded
		}
		return text
	}
	return map[string]any{}
}

func decodeCursorValue(value any) any {
	text, ok := value.(string)
	if !ok {
		return value
	}
	trimmed := strings.TrimSpace(text)
	if len(trimmed) < 2 {
		return text
	}
	if !(strings.HasPrefix(trimmed, "{") && strings.HasSuffix(trimmed, "}")) && !(strings.HasPrefix(trimmed, "[") && strings.HasSuffix(trimmed, "]")) && !(strings.HasPrefix(trimmed, `"`) && strings.HasSuffix(trimmed, `"`)) {
		return text
	}
	var decoded any
	if json.Unmarshal([]byte(trimmed), &decoded) != nil || decoded == nil {
		return text
	}
	if _, still := decoded.(string); still {
		return decodeCursorValue(decoded)
	}
	return decoded
}

func formatCursorResult(value any, depth int) string {
	if depth <= 0 || value == nil {
		return ""
	}
	value = decodeCursorValue(value)
	switch value := value.(type) {
	case string:
		if searchableValue("", value) == nil {
			return ""
		}
		return value
	case []any:
		var parts []string
		for _, item := range value {
			if text := strings.TrimSpace(formatCursorResult(item, depth-1)); text != "" {
				parts = append(parts, text)
			}
		}
		return strings.Join(parts, "\n")
	case map[string]any:
		for _, key := range []string{"content", "contents", "output", "text"} {
			if _, ok := value[key]; !ok {
				continue
			}
			if text := strings.TrimSpace(formatCursorResult(value[key], depth-1)); text != "" {
				return text
			}
		}
		if text := cursorErrorText(value["error"]); text != "" {
			return text
		}
		if nested, ok := value["result"]; ok {
			if text := strings.TrimSpace(formatCursorResult(nested, depth-1)); text != "" {
				return text
			}
		}
		return prettyJSON(searchableValue("", value))
	default:
		return prettyJSON(value)
	}
}

func cursorErrorText(value any) string {
	switch value := value.(type) {
	case string:
		return strings.TrimSpace(value)
	case map[string]any:
		if text := strings.TrimSpace(stringValue(value["message"])); text != "" {
			return text
		}
		if len(value) == 0 {
			return ""
		}
		return prettyJSON(value)
	default:
		return ""
	}
}

func cursorRunMetadata(tool map[string]any, decoded any) map[string]any {
	run := map[string]any{}
	switch strings.ToLower(stringValue(tool["status"])) {
	case "error":
		run["status"] = "error"
	case "cancelled":
		run["status"] = "cancelled"
	}
	if object, ok := decoded.(map[string]any); ok {
		if code, ok := wholeNumber(object["exitCode"]); ok {
			run["result"] = map[string]any{"exitCode": code}
			if code != 0 {
				run["status"] = "error"
			}
		}
	}
	if len(run) == 0 {
		return nil
	}
	return map[string]any{"run": run}
}

func wholeNumber(value any) (int, bool) {
	switch value := value.(type) {
	case float64:
		code := int(value)
		return code, value == float64(code)
	case int:
		return value, true
	default:
		return 0, false
	}
}

func prettyJSON(value any) string {
	if value == nil {
		return ""
	}
	var buf bytes.Buffer
	encoder := json.NewEncoder(&buf)
	encoder.SetEscapeHTML(false)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(value); err != nil {
		return ""
	}
	return strings.TrimRight(buf.String(), "\n")
}

func normalizeLegacyCursor(row cursorRow) ([]domain.Event, []domain.Warning) {
	values := []map[string]any{row.Value}
	if all := objects(row.Value["allComposers"]); len(all) > 0 {
		values = all
	}
	var events []domain.Event
	var warnings []domain.Warning
	for _, value := range values {
		conversation := objects(value["conversation"])
		if len(conversation) == 0 {
			conversation = objects(value["messages"])
		}
		if len(conversation) == 0 {
			warnings = append(warnings, unknown(row.Record, "unsupported legacy Cursor value shape"))
			continue
		}
		for _, message := range conversation {
			id := stringValue(message["bubbleId"])
			role := stringValue(message["role"])
			if role == "" {
				switch numberString(message["type"]) {
				case "1":
					role = "user"
				case "2":
					role = "assistant"
				}
			}
			events = append(events, domain.Event{Key: nativeKey("message", id, message), Kind: "message", Role: role, Text: contentText(message), Sources: source(row.Record.Path, row.Record.Line)})
		}
	}
	return events, warnings
}
