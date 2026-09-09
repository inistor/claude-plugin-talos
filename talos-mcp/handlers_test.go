package main

import (
	"encoding/json"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/siderolabs/talos/pkg/machinery/api/common"
	"github.com/siderolabs/talos/pkg/machinery/api/machine"
	"github.com/siderolabs/talos/pkg/machinery/constants"
)

// A missing "status" used to be read as success, which is how a failed upgrade
// came back with IsError=false and the error text dropped.
func TestStatusAwareResult(t *testing.T) {
	for _, tc := range []struct {
		name    string
		payload map[string]any
		wantErr bool
	}{
		{"ok", map[string]any{"status": "ok"}, false},
		{"missing status", map[string]any{"stages": []string{"drained"}}, true},
		{"empty status", map[string]any{"status": ""}, true},
		{"explicit failure", map[string]any{"status": "install_failed"}, true},
		{"error-suffixed key", map[string]any{"status": "ok", "wait_error": "timeout"}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := statusAwareResult(tc.payload).IsError; got != tc.wantErr {
				t.Errorf("IsError = %v, want %v", got, tc.wantErr)
			}
		})
	}
}

// A plain-text error result must not silently merge as an empty payload.
func TestExtractJSONResultVsResultText(t *testing.T) {
	textErr := mcp.NewToolResultError("image pull failed: no such host")
	if got := extractJSONResult(textErr); len(got) != 0 {
		t.Errorf("extractJSONResult on plain text = %v, want empty", got)
	}
	if got := resultText(textErr); !strings.Contains(got, "no such host") {
		t.Errorf("resultText = %q, want it to preserve the message", got)
	}

	b, _ := json.Marshal(map[string]any{"status": "ok", "api": "lifecycle"})
	jsonRes := mcp.NewToolResultText(string(b))
	got := extractJSONResult(jsonRes)
	if got["status"] != "ok" || got["api"] != "lifecycle" {
		t.Errorf("extractJSONResult on JSON = %v, want the decoded fields", got)
	}
}

// etcd member IDs are uint64 and commonly exceed 2^53.
func TestParseMemberID(t *testing.T) {
	const big = "10501334649042878790" // > 2^53, not exactly representable as float64

	got, err := parseMemberID(big)
	if err != nil {
		t.Fatalf("parseMemberID(%s) errored: %v", big, err)
	}
	if want := uint64(10501334649042878790); got != want {
		t.Errorf("parseMemberID = %d, want %d", got, want)
	}

	// The same value arriving as a JSON number has already lost precision.
	if _, err := parseMemberID(float64(10501334649042878790)); err == nil {
		t.Error("parseMemberID accepted an out-of-range float64; want an error")
	}
	if _, err := parseMemberID(nil); err == nil {
		t.Error("parseMemberID(nil) succeeded; want an error")
	}
	if _, err := parseMemberID("not-a-number"); err == nil {
		t.Error("parseMemberID on a non-numeric string succeeded; want an error")
	}
	if got, err := parseMemberID(float64(42)); err != nil || got != 42 {
		t.Errorf("parseMemberID(42) = %d, %v; want 42, nil", got, err)
	}
}

// The Containers/Stats RPCs take the literal containerd namespace, not the
// short alias used in the tool schema. Talos rejects the CRI inspector for any
// namespace other than "k8s.io".
func TestResolveContainerdNamespace(t *testing.T) {
	for _, tc := range []struct {
		alias      string
		wantNS     string
		wantDriver common.ContainerDriver
	}{
		{"", constants.K8sContainerdNamespace, common.ContainerDriver_CRI},
		{"cri", constants.K8sContainerdNamespace, common.ContainerDriver_CRI},
		{"CRI", constants.K8sContainerdNamespace, common.ContainerDriver_CRI},
		{"system", constants.SystemContainerdNamespace, common.ContainerDriver_CONTAINERD},
	} {
		t.Run(tc.alias, func(t *testing.T) {
			ns, driver := resolveContainerdNamespace(tc.alias)
			if ns != tc.wantNS || driver != tc.wantDriver {
				t.Errorf("= (%q, %v), want (%q, %v)", ns, driver, tc.wantNS, tc.wantDriver)
			}
			if ns == "cri" {
				t.Error(`returned the alias "cri" verbatim; the server rejects it`)
			}
		})
	}
}

// An unrecognised mode must error rather than fall back to AUTO, which reboots.
func TestParseApplyMode(t *testing.T) {
	for _, tc := range []struct {
		in      string
		want    machine.ApplyConfigurationRequest_Mode
		wantErr bool
	}{
		{"", machine.ApplyConfigurationRequest_AUTO, false},
		{"auto", machine.ApplyConfigurationRequest_AUTO, false},
		{"no-reboot", machine.ApplyConfigurationRequest_NO_REBOOT, false},
		{"STAGED", machine.ApplyConfigurationRequest_STAGED, false},
		{"try", machine.ApplyConfigurationRequest_TRY, false},
		{"no_reboot", 0, true},
		{"noreboot", 0, true},
		{"nonsense", 0, true},
	} {
		t.Run(tc.in, func(t *testing.T) {
			got, err := parseApplyMode(tc.in)
			if (err != nil) != tc.wantErr {
				t.Fatalf("err = %v, wantErr = %v", err, tc.wantErr)
			}
			if err == nil && got != tc.want {
				t.Errorf("= %v, want %v", got, tc.want)
			}
		})
	}
}

// fakeStream replays fixed chunks, deliberately splitting lines across them.
type fakeStream struct {
	chunks []string
	i      int
}

func (f *fakeStream) Recv() (*common.Data, error) {
	if f.i >= len(f.chunks) {
		return nil, io.EOF
	}
	c := f.chunks[f.i]
	f.i++
	return &common.Data{Bytes: []byte(c)}, nil
}

// gRPC chunk boundaries do not align to newlines, so filtering per chunk used
// to split a matching line into fragments that each failed the match.
func TestCollectStreamFilterAcrossChunkBoundary(t *testing.T) {
	// "kubelet service failed" is split across three chunks.
	s := &fakeStream{chunks: []string{"noise line\nkube", "let service ", "failed\ntrailing noise\n"}}
	got, err := collectStream(s, "kubelet service failed")
	if err != nil {
		t.Fatalf("collectStream errored: %v", err)
	}
	if got != "kubelet service failed\n" {
		t.Errorf("= %q, want the reassembled line", got)
	}
}

func TestCollectStreamFilterFlushesUnterminatedTail(t *testing.T) {
	s := &fakeStream{chunks: []string{"alpha\nbeta-match"}} // no trailing newline
	got, err := collectStream(s, "beta-match")
	if err != nil {
		t.Fatalf("collectStream errored: %v", err)
	}
	if got != "beta-match" {
		t.Errorf("= %q, want the final unterminated line", got)
	}
}

func TestCollectStreamUnfilteredIsVerbatim(t *testing.T) {
	s := &fakeStream{chunks: []string{"a\nb", "c\n"}}
	got, err := collectStream(s, "")
	if err != nil {
		t.Fatalf("collectStream errored: %v", err)
	}
	if got != "a\nbc\n" {
		t.Errorf("= %q, want the concatenated chunks unchanged", got)
	}
}

type errStream struct{}

func (errStream) Recv() (*common.Data, error) { return nil, errors.New("stream broke") }

func TestCollectStreamPropagatesError(t *testing.T) {
	if _, err := collectStream(errStream{}, ""); err == nil {
		t.Error("collectStream swallowed a stream error")
	}
}

// Results must carry structured content as well as the JSON text fallback:
// the spec prefers structuredContent, but keeping the serialized JSON in a
// text block is what older clients read.
func TestResultsCarryStructuredContent(t *testing.T) {
	res, err := jsonResult(map[string]any{"hostname": "cp-1"})
	if err != nil {
		t.Fatalf("jsonResult errored: %v", err)
	}
	if res.StructuredContent == nil {
		t.Error("jsonResult returned no structured content")
	}
	if resultText(res) == "" {
		t.Error("jsonResult dropped the text fallback")
	}

	ok := statusAwareResult(map[string]any{"status": "ok", "api": "lifecycle"})
	if ok.StructuredContent == nil {
		t.Error("statusAwareResult success path returned no structured content")
	}
	if ok.IsError {
		t.Error("statusAwareResult flagged a success as an error")
	}

	// Failure results stay text-only; NewToolResultError carries IsError and
	// output-schema validation is skipped when there is no structured content.
	bad := statusAwareResult(map[string]any{"status": "install_failed"})
	if !bad.IsError {
		t.Error("statusAwareResult did not flag a failure")
	}
}

// The upgrade output schema must allow every status handlers.go can emit on the
// structured (success) path, or a real upgrade would fail output validation.
func TestUpgradeOutputSchemaCoversEmittedStatuses(t *testing.T) {
	var schema struct {
		Required   []string `json:"required"`
		Properties struct {
			Status struct {
				Enum []string `json:"enum"`
			} `json:"status"`
		} `json:"properties"`
	}
	if err := json.Unmarshal([]byte(upgradeOutputSchema), &schema); err != nil {
		t.Fatalf("upgradeOutputSchema is not valid JSON: %v", err)
	}

	allowed := map[string]bool{}
	for _, s := range schema.Properties.Status.Enum {
		allowed[s] = true
	}

	// Every status set on a talos_upgrade payload in handlers.go.
	for _, status := range []string{
		"ok", "failed", "install_failed", "installed_no_reboot", "rebooted_no_wait",
		"cordon_failed", "drain_failed", "k8s_discovery_failed", "internal_error",
	} {
		if !allowed[status] {
			t.Errorf("status %q is emitted by handlers but missing from the output schema enum", status)
		}
	}
	if len(schema.Required) != 1 || schema.Required[0] != "status" {
		t.Errorf("required = %v, want exactly [status]", schema.Required)
	}
}
