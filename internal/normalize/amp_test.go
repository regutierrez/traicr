package normalize

import (
	"context"
	"encoding/json"
	"testing"
	"testing/fstest"

	"github.com/regutierrez/traicr/internal/domain"
)

func TestAmpPairsNativeToolIDsBeforeProviderIDs(t *testing.T) {
	source := fstest.MapFS{"source/export.json": {Data: []byte(`{"messages":[
 {"id":"m1","role":"assistant","content":[{"type":"tool_use","id":"TU-native","providerToolUseId":"call-provider","name":"bash","input":{"command":"pwd"}}]},
 {"id":"m2","role":"user","content":[{"type":"tool_result","toolUseID":"TU-native","content":"/repo"}]}
 ]}`)}}
	result, err := Run(context.Background(), domain.Descriptor{Harness: "amp"}, source)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Events) != 2 || result.Events[0].CallID != "TU-native" || result.Events[1].CallID != result.Events[0].CallID {
		t.Fatalf("unpaired Amp tool result: %+v", result.Events)
	}
	if result.Version != 2 {
		t.Fatalf("Amp normalizer must rebuild stored ID mappings: version %d", result.Version)
	}
}

func TestAmpStoresParentInMetadataNotParentKey(t *testing.T) {
	source := fstest.MapFS{"source/export.json": {Data: []byte(`{"messages":[
 {"id":"parent","role":"user","content":[{"type":"text","text":"prompt"}]},
 {"id":"child","parentId":"parent","role":"assistant","content":[{"type":"text","text":"reply"}]}
 ]}`)}}
	result, err := Run(context.Background(), domain.Descriptor{Harness: "amp"}, source)
	if err != nil || len(result.Events) != 2 {
		t.Fatalf("Amp parent metadata: %+v %v", result, err)
	}
	if result.Events[0].ParentKey != "" || result.Events[1].ParentKey != "" {
		t.Fatalf("dangling Amp ParentKey: %q %q", result.Events[0].ParentKey, result.Events[1].ParentKey)
	}
	var child struct {
		Message string `json:"transcript_message"`
		Parent  string `json:"transcript_parent"`
	}
	if err := json.Unmarshal(result.Events[1].Metadata, &child); err != nil {
		t.Fatal(err)
	}
	if child.Message != "amp-message:child" || child.Parent != "amp-message:parent" {
		t.Fatalf("Amp transcript parent metadata: %+v", child)
	}
	if result.Events[0].Key != "message:parent:0" || result.Events[1].Key != "message:child:0" {
		t.Fatalf("Amp event keys: %q %q", result.Events[0].Key, result.Events[1].Key)
	}
}

func TestAmpPreservesNativeMessageOrderWithoutIDsOrTimestamps(t *testing.T) {
	source := fstest.MapFS{"source/export.json": {Data: []byte(`{"messages":[
 {"role":"user","meta":{"sentAt":1787140168649},"content":[{"type":"text","text":"First prompt"}]},
 {"role":"assistant","content":[{"type":"thinking","thinking":"Reasoning"},{"type":"text","text":"Response"}]}
 ]}`)}}
	result, err := Run(context.Background(), domain.Descriptor{Harness: "amp"}, source)
	if err != nil || len(result.Events) != 3 {
		t.Fatalf("Amp normalization: %+v %v", result, err)
	}
	var previous string
	for i, event := range result.Events {
		var meta struct {
			Message string `json:"transcript_message"`
			Order   int    `json:"transcript_order"`
			Block   int    `json:"transcript_block"`
		}
		if err := json.Unmarshal(event.Metadata, &meta); err != nil {
			t.Fatal(err)
		}
		if meta.Message == "" || (i == 0 && meta.Order != 0) || (i > 0 && meta.Order != 1) {
			t.Fatalf("native order lost: %+v", meta)
		}
		if i == 2 && (meta.Message != previous || meta.Block != 1) {
			t.Fatalf("assistant content blocks split: %+v", meta)
		}
		previous = meta.Message
	}
}
