package collector

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"strings"

	"github.com/regutierrez/traicr/internal/archive"
	"github.com/regutierrez/traicr/internal/config"
	"github.com/regutierrez/traicr/internal/domain"
)

// UploadStatus describes one step of uploading an archive. Phase "sending"
// reports Sent of Size bytes; the server's streamed phases ("validating",
// "normalizing", "indexing") report Completed of Total traces; "complete"
// marks the archive's import report as received.
type UploadStatus struct {
	File      string
	Phase     string
	Sent      int64
	Size      int64
	Completed int
	Total     int
}

type UploadProgress func(UploadStatus)

func (p UploadProgress) report(status UploadStatus) {
	if p != nil {
		p(status)
	}
}

func Upload(ctx context.Context, client *http.Client, cfg *config.Collector, configPath string, files []string, progress UploadProgress) ([]domain.ImportReport, error) {
	if cfg.ServerURL == "" || cfg.Token == "" {
		return nil, errors.New("not logged in; run traicr login URL and provide a token")
	}
	endpoint, err := importURL(cfg.ServerURL)
	if err != nil {
		return nil, err
	}
	if client == nil {
		client = &http.Client{}
	}
	copyClient := *client
	copyClient.CheckRedirect = func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse }
	var reports []domain.ImportReport
	for _, filePath := range files {
		report, manifest, err := uploadOne(ctx, &copyClient, endpoint, cfg.Token, filePath, progress)
		if err != nil {
			return reports, err
		}
		if report.Machine.ID != manifest.SourceMachine.ID {
			return reports, fmt.Errorf("upload %s: server report belongs to machine %q, archive belongs to %q", filePath, report.Machine.ID, manifest.SourceMachine.ID)
		}
		if err := validateOutcomes(report, manifest.Traces); err != nil {
			return reports, fmt.Errorf("upload %s: %w", filePath, err)
		}
		if manifest.SourceMachine.ID == cfg.MachineID {
			acknowledge(cfg, report)
			if err := config.SaveCollector(configPath, *cfg); err != nil {
				return reports, err
			}
		}
		reports = append(reports, report)
	}
	return reports, nil
}

func importURL(server string) (string, error) {
	parsed, err := url.Parse(server)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
		return "", errors.New("server URL must be an absolute http or https URL")
	}
	if parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
		return "", errors.New("server URL must not contain credentials, a query, or a fragment")
	}
	parsed.Path = path.Join(parsed.Path, "/api/v1/imports")
	return parsed.String(), nil
}

func uploadOne(ctx context.Context, client *http.Client, endpoint, token, filePath string, progress UploadProgress) (domain.ImportReport, domain.Manifest, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return domain.ImportReport{}, domain.Manifest{}, err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return domain.ImportReport{}, domain.Manifest{}, err
	}
	validated, err := archive.Validate(ctx, file, info.Size(), archive.DefaultLimits())
	if err != nil {
		return domain.ImportReport{}, domain.Manifest{}, fmt.Errorf("validate %s before upload: %w", filePath, err)
	}
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return domain.ImportReport{}, domain.Manifest{}, err
	}
	reader := &progressReader{reader: file, file: filePath, total: info.Size(), report: progress}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, reader)
	if err != nil {
		return domain.ImportReport{}, domain.Manifest{}, err
	}
	request.ContentLength = info.Size()
	request.Header.Set("Authorization", "Bearer "+token)
	request.Header.Set("Content-Type", "application/zip")
	response, err := client.Do(request)
	if err != nil {
		return domain.ImportReport{}, domain.Manifest{}, fmt.Errorf("upload %s: %w", filePath, err)
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		message, _ := io.ReadAll(io.LimitReader(response.Body, 4096))
		return domain.ImportReport{}, domain.Manifest{}, fmt.Errorf("upload %s: server returned %s: %s", filePath, response.Status, strings.TrimSpace(string(message)))
	}
	report, err := readCompleteReport(response.Body, filePath, progress)
	if err != nil {
		return domain.ImportReport{}, domain.Manifest{}, fmt.Errorf("upload %s: %w", filePath, err)
	}
	progress.report(UploadStatus{File: filePath, Phase: "complete", Completed: len(report.Traces), Total: len(report.Traces)})
	return report, validated.Manifest, nil
}

func readCompleteReport(reader io.Reader, file string, progress UploadProgress) (domain.ImportReport, error) {
	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 64<<10), 4<<20)
	var report *domain.ImportReport
	for scanner.Scan() {
		var event domain.Progress
		if err := json.Unmarshal(scanner.Bytes(), &event); err != nil {
			return domain.ImportReport{}, fmt.Errorf("malformed import progress: %w", err)
		}
		switch event.Phase {
		case "validating", "normalizing", "indexing":
			if report != nil {
				return domain.ImportReport{}, errors.New("progress received after complete report")
			}
			progress.report(UploadStatus{File: file, Phase: event.Phase, Completed: event.Completed, Total: event.Total})
		case "complete":
			if report != nil || event.Report == nil {
				return domain.ImportReport{}, errors.New("complete progress must contain exactly one report")
			}
			report = event.Report
		case "failed":
			if report != nil {
				return domain.ImportReport{}, errors.New("progress received after complete report")
			}
			if event.Error == "" {
				return domain.ImportReport{}, errors.New("import failed without an error")
			}
			return domain.ImportReport{}, fmt.Errorf("import failed: %s", event.Error)
		default:
			return domain.ImportReport{}, fmt.Errorf("unknown import progress phase %q", event.Phase)
		}
	}
	if err := scanner.Err(); err != nil {
		return domain.ImportReport{}, fmt.Errorf("read import progress: %w", err)
	}
	if report == nil {
		return domain.ImportReport{}, errors.New("import response ended without a complete report")
	}
	return *report, nil
}

func acknowledge(cfg *config.Collector, report domain.ImportReport) {
	if cfg.State == nil {
		cfg.State = map[string]config.CollectionRevision{}
	}
	for _, outcome := range report.Traces {
		if outcome.Status == "imported" || outcome.Status == "updated" || outcome.Status == "unchanged" {
			key := StateKey(outcome.Harness, outcome.NativeTraceID)
			cfg.State[key] = config.CollectionRevision{Digest: outcome.RevisionDigest, MachineID: cfg.MachineID}
		}
	}
}

func validateOutcomes(report domain.ImportReport, descriptors []domain.Descriptor) error {
	expected := make(map[string]string, len(descriptors))
	for _, descriptor := range descriptors {
		expected[StateKey(descriptor.Harness, descriptor.NativeTraceID)] = descriptor.RevisionDigest
	}
	seen := make(map[string]bool, len(report.Traces))
	counts := map[string]int{}
	for _, outcome := range report.Traces {
		key := StateKey(outcome.Harness, outcome.NativeTraceID)
		digest, ok := expected[key]
		if !ok || digest != outcome.RevisionDigest {
			return fmt.Errorf("server reported an unknown trace revision %s/%s", outcome.Harness, outcome.NativeTraceID)
		}
		if seen[key] {
			return fmt.Errorf("server reported trace %s/%s more than once", outcome.Harness, outcome.NativeTraceID)
		}
		seen[key] = true
		switch outcome.Status {
		case "imported", "updated", "unchanged", "partially_parsed", "unsupported", "failed":
			counts[outcome.Status]++
		default:
			return fmt.Errorf("server reported unknown trace status %q", outcome.Status)
		}
	}
	if len(seen) != len(expected) {
		return fmt.Errorf("server reported %d of %d trace outcomes", len(seen), len(expected))
	}
	if counts["imported"] != report.Imported || counts["updated"] != report.Updated || counts["unchanged"] != report.Unchanged || counts["partially_parsed"] != report.Partial || counts["unsupported"] != report.Unsupported || counts["failed"] != report.Failed {
		return errors.New("server report totals do not match its trace outcomes")
	}
	return nil
}

type progressReader struct {
	reader io.Reader
	file   string
	total  int64
	sent   int64
	report UploadProgress
}

func (reader *progressReader) Read(data []byte) (int, error) {
	read, err := reader.reader.Read(data)
	reader.sent += int64(read)
	if read > 0 {
		reader.report.report(UploadStatus{File: reader.file, Phase: "sending", Sent: reader.sent, Size: reader.total})
	}
	return read, err
}
