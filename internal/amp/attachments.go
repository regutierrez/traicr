package amp

import (
	"crypto/sha256"
	"fmt"
	"net/url"
	"strings"

	"github.com/regutierrez/traicr/internal/domain"
)

func AttachmentLocation(rawURL string) (downloadURL, archivePath string) {
	u, err := url.Parse(rawURL)
	if err != nil || u.Scheme != "https" || u.Host != "ampcode.com" || u.User != nil || u.RawQuery != "" || u.ForceQuery {
		return "", ""
	}
	name, ok := strings.CutPrefix(u.EscapedPath(), "/user-content/attachments/")
	if !ok || name == "" || name == "." || name == ".." || strings.ContainsAny(name, "/\\%") {
		return "", ""
	}
	u.Fragment, u.RawFragment = "", ""
	downloadURL = u.String()
	return downloadURL, fmt.Sprintf("source/attachments/%x", sha256.Sum256([]byte(downloadURL)))
}

func Attachments(value any, pointer string) []domain.Attachment {
	var attachments []domain.Attachment
	switch value := value.(type) {
	case []any:
		for index, item := range value {
			attachments = append(attachments, Attachments(item, fmt.Sprintf("%s/%d", pointer, index))...)
		}
	case map[string]any:
		if kind := value["type"]; kind == "image" || kind == "attachment" {
			source, _ := value["source"].(map[string]any)
			attachment := domain.Attachment{
				Name: attachmentString(value, "name", "filename"), Path: attachmentString(value, "sourcePath", "path"), URL: attachmentString(value, "url"),
				MediaType: attachmentString(value, "mimeType", "media_type"), SourcePointer: pointer,
			}
			if attachment.URL == "" {
				attachment.URL = attachmentString(source, "url")
			}
			if attachment.URL == "" {
				attachment.URL, _ = AttachmentLocation(attachment.Path)
			}
			if attachment.MediaType == "" {
				attachment.MediaType = attachmentString(source, "media_type", "mimeType")
			}
			attachment.Inline = source["type"] == "base64"
			attachments = append(attachments, attachment)
		}
		for _, key := range []string{"messages", "thread", "events", "content", "run", "result", "progress", "displayImages"} {
			if nested, ok := value[key]; ok {
				attachments = append(attachments, Attachments(nested, pointer+"/"+key)...)
			}
		}
	}
	return attachments
}

func attachmentString(value map[string]any, keys ...string) string {
	for _, key := range keys {
		if found, _ := value[key].(string); found != "" {
			return found
		}
	}
	return ""
}
