package normalize

import "testing/fstest"

const piV3Records = `{"type":"session","version":3,"id":"session-fixture","timestamp":"2026-01-01T00:00:00Z","cwd":"/workspace/example"}
{"type":"message","id":"reply","parentId":"prompt","timestamp":"2026-01-01T00:00:02Z","message":{"role":"assistant","provider":"example-provider","model":"example-model","content":[{"type":"thinking","thinking":"Check the fixture."},{"type":"toolCall","id":"call-1","name":"read","arguments":{"path":"fixture.txt"}}]}}
{"type":"message","id":"prompt","parentId":null,"timestamp":"2026-01-01T00:00:01Z","message":{"role":"user","content":"Read the fixture."}}
{"type":"message","id":"result","parentId":"reply","timestamp":"2026-01-01T00:00:03Z","message":{"role":"toolResult","toolCallId":"call-1","toolName":"read","content":"fixture contents"}}
{"type":"model_change","id":"model","parentId":"result","timestamp":"2026-01-01T00:00:04Z","provider":"example-provider","modelId":"example-model-2"}
{"type":"compaction","id":"compact","parentId":"model","timestamp":"2026-01-01T00:00:05Z","summary":"Earlier fixture activity summarized.","firstKeptEntryId":"result","tokensBefore":100}
{"type":"branch_summary","id":"branch","parentId":"prompt","timestamp":"2026-01-01T00:00:06Z","fromId":"prompt","summary":"Alternate fixture branch."}
`

const claudeCodeRecords = `{"type":"user","uuid":"u1","parentUuid":null,"sessionId":"fixture","timestamp":"2026-01-02T00:00:00Z","message":{"role":"user","content":[{"type":"text","text":"Inspect fixture.txt"},{"type":"image","name":"fixture.png","source":{"type":"base64","media_type":"image/png","data":"c2FuaXRpemVkLWZpeHR1cmU="}}]}}
{"type":"assistant","uuid":"a1","parentUuid":"u1","sessionId":"fixture","timestamp":"2026-01-02T00:00:01Z","message":{"role":"assistant","model":"example-model","content":[{"type":"thinking","thinking":"I should inspect it."},{"type":"tool_use","id":"tool-1","name":"Read","input":{"file_path":"fixture.txt"}}]}}
{"type":"user","uuid":"r1","parentUuid":"a1","sessionId":"fixture","timestamp":"2026-01-02T00:00:02Z","message":{"role":"user","content":[{"type":"tool_result","tool_use_id":"tool-1","content":"fixture contents"}]}}
`

const ampRichExport = `{
  "v": 84,
  "id": "T-amp-rich-fixture",
  "title": "Inspect the failing build",
  "created": 1789552800000,
  "updatedAt": "2026-09-16T10:04:00Z",
  "archived": true,
  "agentMode": "high",
  "activatedSkills": [{"name": "tdd"}],
  "env": {"initial": {"workingDirectory": "/repo", "platform": {"os": "linux", "clientVersion": "fixture"}, "trees": [{"uri": "file:///repo", "repository": {"url": "https://github.com/example/project", "ref": "main", "sha": "fixture-commit", "type": "git"}}]}},
  "meta": {"features": ["fast"], "executorType": "orb"},
  "messages": [
    {"messageId": 0, "role": "user", "createdAt": "2026-09-16T10:00:00Z", "meta": {"sentAt": 1789552800000}, "content": [{"type": "text", "text": "Inspect the build and this screenshot."}, {"type": "image", "source": {"type": "url", "url": "https://ampcode.com/attachments/fixture.png"}, "sourcePath": "screenshot.png"}]},
    {"messageId": 1, "role": "assistant", "createdAt": "2026-09-16T10:00:01Z", "meta": {"openAIResponsePhase": "commentary"}, "state": {"type": "complete", "stopReason": "tool_use"}, "usage": {"model": "example-model", "inputTokens": 12, "outputTokens": 7, "totalInputTokens": 112, "cacheReadInputTokens": 100, "cacheCreationInputTokens": 0, "maxInputTokens": 1000}, "content": [{"type": "thinking", "provider": "example-provider", "thinking": "Check the failing command first.", "startTime": 1789552801000, "finalTime": 1789552801500, "signature": "PRIVATE_SIGNATURE"}, {"type": "text", "text": "I will inspect the failure.", "startTime": 1789552801600, "finalTime": 1789552802000}, {"type": "tool_use", "id": "TU-shell", "providerToolUseId": "provider-shell", "name": "shell_command", "input": {"command": "go test ./...", "workdir": "/repo"}, "complete": true}]},
    {"messageId": 2, "role": "user", "createdAt": "2026-09-16T10:00:03Z", "content": [{"type": "tool_result", "toolUseID": "TU-shell", "run": {"status": "done", "result": {"output": "FAIL: expected 3, got 2", "exitCode": 1}}}]},
    {"messageId": 3, "role": "assistant", "content": [{"type": "tool_use", "id": "TU-oracle", "name": "oracle", "input": {"task": "Explain the failing assertion"}}]},
    {"messageId": 4, "role": "user", "content": [{"type": "tool_result", "toolUseID": "TU-oracle", "run": {"status": "done", "result": "**Diagnosis:** the upper bound is exclusive."}}]},
    {"messageId": 5, "role": "info", "content": [{"type": "summary", "summary": {"type": "message", "summary": "The build fails because the upper bound is exclusive. Preserve the boundary case."}}]},
    {"messageId": 6, "role": "assistant", "content": [{"type": "tool_use", "id": "TU-patch", "name": "apply_patch", "input": {"patchText": "*** Begin Patch\n*** Update File: range.go\n@@\n-return n < end\n+return n <= end\n*** End Patch"}}]},
    {"messageId": 7, "role": "user", "content": [{"type": "tool_result", "toolUseID": "TU-patch", "run": {"status": "done", "result": {"summary": "Updated range.go", "files": [{"uri": "file:///repo/range.go", "type": "update", "additions": 1, "deletions": 1, "diff": "-return n < end\n+return n <= end"}]}}}]},
    {"messageId": 8, "role": "user", "meta": {"fromExecutorThreadID": "T-child-fixture", "fromAutomation": true}, "content": [{"type": "text", "text": "Cross-thread context", "hidden": true}, {"type": "text", "text": "The child checked the other boundary."}]},
    {"messageId": 9, "role": "assistant", "state": {"type": "cancelled"}, "content": [{"type": "text", "text": "Verification was interrupted."}]},
    {"messageId": 10, "role": "assistant", "content": [{"type": "tool_use", "id": "TU-error", "name": "skill", "input": {"name": "missing-skill"}}]},
    {"messageId": 11, "role": "user", "content": [{"type": "tool_result", "toolUseID": "TU-error", "run": {"status": "error", "error": {"message": "Skill not found"}}}]},
    {"messageId": 12, "role": "assistant", "content": [{"type": "tool_use", "id": "TU-child", "name": "create_thread", "input": {"prompt": "Check the boundary"}}]},
    {"messageId": 13, "role": "user", "content": [{"type": "tool_result", "toolUseID": "TU-child", "run": {"status": "done", "result": {"threadID": "T-child-fixture", "url": "https://ampcode.com/threads/T-child-fixture", "title": "Check the boundary"}}}]},
    {"messageId": 14, "role": "assistant", "meta": {"openAIResponsePhase": "final_answer"}, "state": {"type": "complete", "stopReason": "end_turn"}, "usage": {"model": "second-model", "inputTokens": 5, "outputTokens": 11, "cacheReadInputTokens": 0}, "content": [{"type": "text", "text": "The boundary is fixed. Verification still needs to be rerun."}]},
    {"messageId": 15, "role": "info", "content": [{"type": "future_widget", "label": "Unknown content stays inspectable", "bytes": "PRIVATE_BYTES"}]}
  ]
}`

func recordsFS(records string) fstest.MapFS {
	return fstest.MapFS{"source/records.jsonl": {Data: []byte(records)}}
}

func exportFS(export string) fstest.MapFS {
	return fstest.MapFS{"source/export.json": {Data: []byte(export)}}
}
