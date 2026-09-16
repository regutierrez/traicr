package server

import (
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/regutierrez/traicr/internal/amp"
	"github.com/regutierrez/traicr/internal/domain"
)

func (app *application) transcriptAPI(w http.ResponseWriter, r *http.Request) {
	id, ok := requestID(w, r)
	if !ok {
		return
	}
	limit, ok := pageLimit(w, r)
	if !ok {
		return
	}
	var revision int64
	if value := r.URL.Query().Get("revision"); value != "" {
		var err error
		revision, err = strconv.ParseInt(value, 10, 64)
		if err != nil || revision < 0 {
			writeError(w, 400, "invalid_revision", "revision must be a nonnegative integer")
			return
		}
	}
	page, err := app.store.TranscriptEvents(r.Context(), id, revision, r.URL.Query().Get("cursor"), limit)
	if err != nil {
		app.failure(w, err)
		return
	}
	writeJSON(w, page)
}

func (app *application) resolveTrace(w http.ResponseWriter, r *http.Request) {
	id, err := app.store.TraceID(r.Context(), "amp", r.URL.Query().Get("native_id"))
	if errors.Is(err, sql.ErrNoRows) {
		writeError(w, 404, "trace_not_collected", "This Amp trace has not been collected. Use the original Amp link to inspect it.")
		return
	}
	if err != nil {
		app.failure(w, err)
		return
	}
	http.Redirect(w, r, "/traces/"+strconv.FormatInt(id, 10), http.StatusSeeOther)
}

func (app *application) ampSourceBlock(w http.ResponseWriter, r *http.Request, id int64) (any, bool) {
	pointer := r.URL.Query().Get("pointer")
	if !strings.HasPrefix(pointer, "/") {
		writeError(w, 400, "invalid_pointer", "source pointer must start with /")
		return nil, false
	}
	sources, err := app.store.RevisionSources(r.Context(), id)
	if err != nil {
		app.failure(w, err)
		return nil, false
	}
	for _, source := range sources {
		if source.Path == "source/export.json" && source.Size > domain.MaxAmpExportBytes {
			writeError(w, 413, "source_too_large", "Source exceeds the 512 MiB Amp limit. Download the native export instead.")
			return nil, false
		}
	}
	file, err := app.store.OpenSource(r.Context(), id, "source/export.json")
	if err != nil {
		app.failure(w, err)
		return nil, false
	}
	defer file.Close()
	var value any
	if err := json.NewDecoder(io.LimitReader(file, domain.MaxAmpExportBytes)).Decode(&value); err != nil {
		writeError(w, 422, "source_unavailable", "Source JSON is invalid or exceeds the 512 MiB Amp limit. Download the native export instead.")
		return nil, false
	}
	for _, key := range strings.Split(pointer[1:], "/") {
		key = strings.ReplaceAll(strings.ReplaceAll(key, "~1", "/"), "~0", "~")
		switch item := value.(type) {
		case map[string]any:
			value = item[key]
		case []any:
			index, err := strconv.Atoi(key)
			if err != nil || index < 0 || index >= len(item) {
				value = nil
			} else {
				value = item[index]
			}
		default:
			value = nil
		}
		if value == nil {
			writeError(w, 404, "block_not_found", "Source block not found")
			return nil, false
		}
	}
	return value, true
}

func (app *application) sourceBlock(w http.ResponseWriter, r *http.Request) {
	id, ok := requestID(w, r)
	if !ok {
		return
	}
	value, ok := app.ampSourceBlock(w, r, id)
	if !ok {
		return
	}
	w.Header().Set("Content-Type", "application/json")
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	_ = encoder.Encode(value)
}

func (app *application) sourceAttachment(w http.ResponseWriter, r *http.Request) {
	id, ok := requestID(w, r)
	if !ok {
		return
	}
	value, ok := app.ampSourceBlock(w, r, id)
	if !ok {
		return
	}
	block, _ := value.(map[string]any)
	source, _ := block["source"].(map[string]any)
	encoded, _ := source["data"].(string)
	var data []byte
	if source["type"] == "base64" && encoded != "" {
		if len(encoded) > base64.StdEncoding.EncodedLen(10<<20) {
			writeError(w, 413, "attachment_too_large", "Image exceeds the 10 MiB preview limit. Download the native export instead.")
			return
		}
		var err error
		data, err = base64.StdEncoding.DecodeString(encoded)
		if err != nil {
			writeError(w, 422, "invalid_attachment", "Attachment data is not valid base64")
			return
		}
	} else {
		attachments := amp.Attachments(block, "")
		path := ""
		if len(attachments) > 0 && attachments[0].SourcePointer == "" {
			_, path = amp.AttachmentLocation(attachments[0].URL)
		}
		if path == "" {
			writeError(w, 404, "attachment_not_archived", "Attachment bytes are not archived. The server does not fetch external URLs.")
			return
		}
		file, err := app.store.OpenSource(r.Context(), id, path)
		if err != nil {
			app.failure(w, err)
			return
		}
		defer file.Close()
		data, err = io.ReadAll(io.LimitReader(file, (10<<20)+1))
		if err != nil {
			app.failure(w, err)
			return
		}
	}
	if len(data) > 10<<20 {
		writeError(w, 413, "attachment_too_large", "Image exceeds the 10 MiB preview limit. Use Download image or download the native export for inline images.")
		return
	}
	mediaType := http.DetectContentType(data)
	switch mediaType {
	case "image/png", "image/jpeg", "image/gif", "image/webp":
		w.Header().Set("Content-Type", mediaType)
	default:
		writeError(w, 415, "unsupported_preview", "Only PNG, JPEG, GIF and WebP images can be previewed. Inspect the source block for other attachments.")
		return
	}
	if r.URL.Query().Get("download") == "1" {
		w.Header().Set("Content-Disposition", `attachment; filename="attachment"`)
	}
	_, _ = w.Write(data)
}
