package amp

import "testing"

func TestAttachmentLocationOnlyAcceptsAmpFiles(t *testing.T) {
	const valid = "https://ampcode.com/user-content/attachments/fixture.png"
	for _, input := range []string{valid, valid + "#amp-media-width=640"} {
		url, path := AttachmentLocation(input)
		if url != valid || path != "source/attachments/582a1f5b341842222df56057a4c2851406c193ab079e6c4f1237d3518fc46e9c" {
			t.Fatalf("attachment location: %q %q", url, path)
		}
	}
	for _, input := range []string{
		"http://ampcode.com/user-content/attachments/fixture.png",
		"https://ampcode.com.evil.test/user-content/attachments/fixture.png",
		"https://ampcode.com@evil.test/user-content/attachments/fixture.png",
		"https://user@ampcode.com/user-content/attachments/fixture.png",
		"https://ampcode.com:443/user-content/attachments/fixture.png",
		"https://ampcode.com/user-content/attachments/../secret",
		"https://ampcode.com/user-content/attachments/%2e%2e%2fsecret",
		"https://ampcode.com/user-content/attachments/a%2fb.png",
		"https://ampcode.com/user-content/attachments/..",
		"https://ampcode.com/user-content/attachments/",
		valid + "?redirect=https://evil.test",
		"file:///private.png",
		"/private.png",
	} {
		if url, path := AttachmentLocation(input); url != "" || path != "" {
			t.Fatalf("unsafe attachment accepted: %q", input)
		}
	}
}
