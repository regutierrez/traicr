package normalize

import (
	"cmp"
	"encoding/json"
	"maps"
	"slices"
	"strings"

	"github.com/regutierrez/traicr/internal/domain"
)

// Patch is one file edit a tool call made. Patches are derived from a
// Trace's stored Events when they are read, so old traces get them without
// being collected or normalized again.
type Patch struct {
	Path string `json:"path"`
	Tool string `json:"tool"`
	// EventID is the tool call Event that made the edit.
	EventID   int64  `json:"event_id"`
	Timestamp string `json:"timestamp,omitempty"`
	// Operation is edit, write (the whole file was written), or delete.
	Operation string `json:"operation"`
	// Diff holds unified diff hunks without file headers. A hunk header
	// without line numbers, such as a bare "@@", means the call did not
	// record where in the file the hunk is.
	Diff      string `json:"diff,omitempty"`
	Additions int    `json:"additions"`
	Deletions int    `json:"deletions"`
	// Arguments and Result are the call's own text, kept when no diff could be built.
	Arguments string `json:"arguments,omitempty"`
	Result    string `json:"result,omitempty"`
	// Failed is true when the result says the edit did not apply.
	Failed bool `json:"failed,omitempty"`
}

// editTools are the lowercase tool names that change files, across harnesses.
var editTools = map[string]string{
	"edit": "edit", "edit_file": "edit", "edit_file_v2": "edit", "multiedit": "edit", "strreplace": "edit",
	"str_replace": "edit", "search_replace": "edit", "apply_patch": "edit", "apply-patch": "edit", "patch": "edit",
	"write": "write", "write_file": "write", "create_file": "write",
	"delete": "delete", "delete_file": "delete",
}

// EditTools lists the tool names Patches reads, so a store can fetch only
// those calls and their results.
func EditTools() []string {
	return slices.Collect(maps.Keys(editTools))
}

// Patches returns the file edits made by the tool calls in events, which are
// a Trace's tool calls and results in transcript order. Other events are ignored.
func Patches(events []domain.Event) []Patch {
	results := map[string]domain.Event{}
	for _, event := range events {
		if event.Kind == "tool_result" && event.CallID != "" {
			results[event.CallID] = event
		}
	}
	patches := []Patch{}
	for _, event := range events {
		operation, ok := editTools[strings.ToLower(event.Tool)]
		if event.Kind != "tool_call" || !ok {
			continue
		}
		result, hasResult := results[event.CallID]
		// Typed fields keep the decoder from building maps for the rest of a
		// result, which can hold whole files.
		var metadata struct {
			Run struct {
				Status string         `json:"status"`
				Result recordedResult `json:"result"`
			} `json:"run"`
		}
		if hasResult {
			_ = json.Unmarshal(result.Metadata, &metadata)
		}
		run := metadata.Run
		// A result that lists its files names every path and diff, so the
		// arguments, which hold the whole patch text, are not decoded.
		var args map[string]any
		if len(run.Result.Files) == 0 {
			_ = json.Unmarshal([]byte(event.Text), &args)
		}
		base := Patch{Tool: event.Tool, EventID: event.ID, Timestamp: event.Timestamp, Operation: operation, Path: argPath(args)}
		switch run.Status {
		case "error", "cancelled", "rejected-by-user":
			base.Failed = true
		}
		// Claude Code marks a failed call only in the result text.
		if strings.HasPrefix(strings.TrimSpace(result.Text), "<tool_use_error>") {
			base.Failed = true
		}
		edits := callPatches(base, args, run.Result)
		if len(edits) == 0 {
			if base.Path == "" {
				continue
			}
			base.Arguments, base.Result = event.Text, result.Text
			edits = []Patch{base}
		}
		patches = append(patches, edits...)
	}
	return patches
}

// recordedResult is the part of a tool result that holds a diff the harness
// recorded. Results of another shape, such as plain text, leave it empty:
// json.Unmarshal skips a field of the wrong type and still fills the others.
type recordedResult struct {
	Diff  string `json:"diff"`
	Files []struct {
		Path string `json:"path"`
		URI  string `json:"uri"`
		Type string `json:"type"`
		Diff string `json:"diff"`
	} `json:"files"`
}

// callPatches builds the patches one call made, preferring a diff the harness
// recorded over one rebuilt from the call's arguments. It returns nil when
// the call carries nothing to diff.
func callPatches(base Patch, args map[string]any, result recordedResult) []Patch {
	// Amp's apply_patch answers with a diff per file it changed.
	if len(result.Files) > 0 {
		patches := []Patch{}
		for _, file := range result.Files {
			patch := base
			patch.Path = strings.TrimPrefix(cmp.Or(file.Path, file.URI), "file://")
			switch file.Type {
			case "add":
				patch.Operation = "write"
			case "delete":
				patch.Operation = "delete"
			}
			patch.setDiff(hunks(file.Diff))
			patches = append(patches, patch)
		}
		return patches
	}
	// Amp's edit_file answers with one fenced unified diff.
	if result.Diff != "" {
		base.setDiff(hunks(result.Diff))
		return []Patch{base}
	}
	// Codex lists its file changes; an added or deleted file's diff is the whole file.
	if changes := objects(args["changes"]); len(changes) > 0 {
		patches := []Patch{}
		for _, change := range changes {
			patch := base
			patch.Path = stringValue(change["path"])
			diff := stringValue(change["diff"])
			switch stringValue(object(change["kind"])["type"]) {
			case "add":
				patch.Operation = "write"
				diff = prefixLines("+", diff)
			case "delete":
				patch.Operation = "delete"
				diff = prefixLines("-", diff)
			default:
				diff = hunks(diff)
			}
			patch.setDiff(diff)
			patches = append(patches, patch)
		}
		return patches
	}
	for _, key := range []string{"patchText", "patch", "input"} {
		if text := stringValue(args[key]); strings.HasPrefix(strings.TrimSpace(text), "*** Begin Patch") {
			return patchTextPatches(base, text)
		}
	}
	// Some edit tools take several replacements in one call.
	replacements := objects(args["edits"])
	if len(replacements) == 0 {
		replacements = []map[string]any{args}
	}
	var diff []string
	for _, replacement := range replacements {
		before, hasBefore := firstValue(replacement, "old_string", "old_str", "oldText", "oldString").(string)
		after, hasAfter := firstValue(replacement, "new_string", "new_str", "newText", "newString").(string)
		if hasBefore && hasAfter {
			diff = append(diff, "@@")
			diff = append(diff, lineDiff(splitLines(before), splitLines(after))...)
		}
	}
	if len(diff) > 0 {
		base.setDiff(strings.Join(diff, "\n"))
		return []Patch{base}
	}
	if content, ok := firstValue(args, "content", "contents", "file_text").(string); ok && base.Operation == "write" {
		base.setDiff(prefixLines("+", content))
		return []Patch{base}
	}
	if base.Operation == "delete" && base.Path != "" {
		return []Patch{base}
	}
	return nil
}

func argPath(args map[string]any) string {
	return firstString(args, "file_path", "path", "filePath", "relativeWorkspacePath", "target_file")
}

func (patch *Patch) setDiff(diff string) {
	patch.Diff = diff
	for _, line := range strings.Split(diff, "\n") {
		switch {
		case strings.HasPrefix(line, "+"):
			patch.Additions++
		case strings.HasPrefix(line, "-"):
			patch.Deletions++
		}
	}
}

// hunks strips a code fence and the file headers before the first hunk from
// a unified diff.
func hunks(diff string) string {
	lines := splitLines(diff)
	if len(lines) > 0 && strings.HasPrefix(lines[0], "```") {
		lines = lines[1:]
	}
	if len(lines) > 0 && strings.HasPrefix(lines[len(lines)-1], "```") {
		lines = lines[:len(lines)-1]
	}
	for index, line := range lines {
		if strings.HasPrefix(line, "@@") {
			return strings.Join(lines[index:], "\n")
		}
	}
	return strings.Join(lines, "\n")
}

// patchTextPatches reads the "*** Begin Patch" format that apply_patch tools
// take: one section per file, with hunks under Update File.
func patchTextPatches(base Patch, text string) []Patch {
	patches := []Patch{}
	var current *Patch
	var lines []string
	flush := func() {
		if current != nil {
			current.setDiff(strings.Join(lines, "\n"))
			patches = append(patches, *current)
		}
		current, lines = nil, nil
	}
	sections := map[string]string{"*** Update File: ": "edit", "*** Add File: ": "write", "*** Delete File: ": "delete"}
	for _, line := range splitLines(text) {
		if strings.HasPrefix(line, "*** ") {
			for prefix, operation := range sections {
				if path, ok := strings.CutPrefix(line, prefix); ok {
					flush()
					patch := base
					patch.Path, patch.Operation = strings.TrimSpace(path), operation
					current = &patch
				}
			}
			if path, ok := strings.CutPrefix(line, "*** Move to: "); ok && current != nil {
				current.Path = strings.TrimSpace(path)
			}
		} else if current != nil {
			lines = append(lines, line)
		}
	}
	flush()
	return patches
}

func prefixLines(prefix, text string) string {
	return strings.Join(prefixed(prefix, splitLines(text)), "\n")
}

// splitLines splits text into lines, without an empty line after a final newline.
func splitLines(text string) []string {
	if text == "" {
		return nil
	}
	return strings.Split(strings.TrimSuffix(text, "\n"), "\n")
}

// maxDiffCells bounds the table lineDiff fills. Larger replacements show as
// every old line removed and every new line added.
const maxDiffCells = 4_000_000

// lineDiff returns unified diff lines that turn before into after, keeping
// the lines they share as context.
func lineDiff(before, after []string) []string {
	if len(before)*len(after) > maxDiffCells {
		return append(prefixed("-", before), prefixed("+", after)...)
	}
	// common[i][j] is the longest common subsequence of before[i:] and after[j:].
	common := make([][]int, len(before)+1)
	for i := range common {
		common[i] = make([]int, len(after)+1)
	}
	for i := len(before) - 1; i >= 0; i-- {
		for j := len(after) - 1; j >= 0; j-- {
			if before[i] == after[j] {
				common[i][j] = common[i+1][j+1] + 1
			} else {
				common[i][j] = max(common[i+1][j], common[i][j+1])
			}
		}
	}
	lines := make([]string, 0, len(before)+len(after))
	i, j := 0, 0
	for i < len(before) || j < len(after) {
		switch {
		case i < len(before) && j < len(after) && before[i] == after[j]:
			lines = append(lines, " "+before[i])
			i, j = i+1, j+1
		case i < len(before) && (j == len(after) || common[i+1][j] >= common[i][j+1]):
			lines = append(lines, "-"+before[i])
			i++
		default:
			lines = append(lines, "+"+after[j])
			j++
		}
	}
	return lines
}

func prefixed(prefix string, lines []string) []string {
	out := make([]string, len(lines))
	for index, line := range lines {
		out[index] = prefix + line
	}
	return out
}
