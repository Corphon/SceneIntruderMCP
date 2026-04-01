// internal/services/video_provider_dashscope.go
package services

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net"
	"net/http"
	"net/url"
	"path"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/Corphon/SceneIntruderMCP/internal/models"
)

var (
	ErrDashScopeVideoEndpointRequired = errors.New("dashscope video endpoint required")
	ErrDashScopeVideoAPIKeyRequired   = errors.New("dashscope video api key required")
	ErrDashScopeVideoTaskIDRequired   = errors.New("dashscope video task id required")
)

type DashScopeVideoProvider struct {
	Endpoint         string
	APIKey           string
	SynthesisPath    string
	TaskPathPrefix   string
	UploadPolicyPath string
	UploadCategory   string
	Client           *http.Client
	PollEvery        time.Duration
	PollAttempts     int
	ModelKeyMapping  map[string]string
}

func NewDashScopeVideoProvider(endpoint string, apiKey string) *DashScopeVideoProvider {
	return &DashScopeVideoProvider{
		Endpoint:         strings.TrimRight(strings.TrimSpace(endpoint), "/"),
		APIKey:           strings.TrimSpace(apiKey),
		SynthesisPath:    "/services/aigc/video-generation/video-synthesis",
		TaskPathPrefix:   "/tasks",
		UploadPolicyPath: "/uploads",
		UploadCategory:   "video_generation",
		PollEvery:        2 * time.Second,
		PollAttempts:     90,
		ModelKeyMapping:  map[string]string{},
	}
}

func (p *DashScopeVideoProvider) httpClient() *http.Client {
	if p.Client != nil {
		return p.Client
	}
	return &http.Client{Timeout: 60 * time.Second}
}

func (p *DashScopeVideoProvider) ProviderPollInterval() time.Duration {
	return p.PollEvery
}

func (p *DashScopeVideoProvider) ProviderMaxPollAttempts() int {
	return p.PollAttempts
}

func (p *DashScopeVideoProvider) SubmitClipTask(ctx context.Context, req models.VideoClipRequest) (*models.VideoProviderTask, error) {
	if strings.TrimSpace(p.APIKey) == "" {
		return nil, ErrDashScopeVideoAPIKeyRequired
	}
	u, err := p.synthesisURL()
	if err != nil {
		return nil, err
	}
	payload, err := json.Marshal(p.buildRequestBody(req))
	if err != nil {
		return nil, err
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, u, bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+p.APIKey)
	httpReq.Header.Set("X-DashScope-Async", "enable")
	if requiresOSSResourceResolve(req.ReferenceImageURL, req.AudioURL) {
		httpReq.Header.Set("X-DashScope-OssResourceResolve", "enable")
	}
	resp, err := p.httpClient().Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("dashscope video submit failed: status=%d body=%s", resp.StatusCode, string(bytes.TrimSpace(body)))
	}
	return p.parseTaskResponse(body)
}

func (p *DashScopeVideoProvider) UploadReferenceImage(ctx context.Context, req models.VideoReferenceUploadRequest) (*models.VideoReferenceUploadResult, error) {
	if strings.TrimSpace(p.APIKey) == "" {
		return nil, ErrDashScopeVideoAPIKeyRequired
	}
	policy, err := p.getUploadPolicy(ctx, req)
	if err != nil {
		return nil, err
	}
	objectKey := buildUploadObjectKey(policy.Output.Dir, policy.Output.UUID, req.FileName, req.ContentType)
	if err := p.uploadToOSS(ctx, policy.Output.Host, objectKey, req, policy); err != nil {
		return nil, err
	}
	bucket := bucketFromOSSHost(policy.Output.Host)
	storagePath := "oss://" + bucket + "/" + strings.TrimLeft(objectKey, "/")
	return &models.VideoReferenceUploadResult{URL: storagePath, StoragePath: storagePath}, nil
}

func (p *DashScopeVideoProvider) PollTask(ctx context.Context, taskID string) (*models.VideoProviderTask, error) {
	taskID = strings.TrimSpace(taskID)
	if taskID == "" {
		return nil, ErrDashScopeVideoTaskIDRequired
	}
	if strings.TrimSpace(p.APIKey) == "" {
		return nil, ErrDashScopeVideoAPIKeyRequired
	}
	u, err := p.taskURL(taskID)
	if err != nil {
		return nil, err
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Authorization", "Bearer "+p.APIKey)
	resp, err := p.httpClient().Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("dashscope video poll failed: status=%d body=%s", resp.StatusCode, string(bytes.TrimSpace(body)))
	}
	return p.parseTaskResponse(body)
}

func (p *DashScopeVideoProvider) synthesisURL() (string, error) {
	base := strings.TrimSpace(p.Endpoint)
	if base == "" {
		return "", ErrDashScopeVideoEndpointRequired
	}
	u, err := url.Parse(base)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") {
		return "", ErrDashScopeVideoEndpointRequired
	}
	synthesisPath := strings.TrimSpace(p.SynthesisPath)
	if synthesisPath == "" {
		synthesisPath = "/services/aigc/video-generation/video-synthesis"
	}
	u.Path = path.Join(u.Path, synthesisPath)
	return u.String(), nil
}

func (p *DashScopeVideoProvider) taskURL(taskID string) (string, error) {
	base := strings.TrimSpace(p.Endpoint)
	if base == "" {
		return "", ErrDashScopeVideoEndpointRequired
	}
	u, err := url.Parse(base)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") {
		return "", ErrDashScopeVideoEndpointRequired
	}
	taskPathPrefix := strings.TrimSpace(p.TaskPathPrefix)
	if taskPathPrefix == "" {
		taskPathPrefix = "/tasks"
	}
	u.Path = path.Join(u.Path, taskPathPrefix, taskID)
	return u.String(), nil
}

func (p *DashScopeVideoProvider) getUploadPolicy(ctx context.Context, req models.VideoReferenceUploadRequest) (dashScopeUploadPolicyResponse, error) {
	var errs []string
	for _, candidate := range p.uploadPolicyModels(req.Model) {
		policyURL, err := p.uploadPolicyURL(candidate)
		if err != nil {
			return dashScopeUploadPolicyResponse{}, err
		}
		httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, policyURL, nil)
		if err != nil {
			return dashScopeUploadPolicyResponse{}, err
		}
		httpReq.Header.Set("Authorization", "Bearer "+p.APIKey)
		resp, err := p.httpClient().Do(httpReq)
		if err != nil {
			errs = append(errs, fmt.Sprintf("model=%s err=%v", candidate, err))
			continue
		}
		body, readErr := io.ReadAll(resp.Body)
		resp.Body.Close()
		if readErr != nil {
			errs = append(errs, fmt.Sprintf("model=%s read_err=%v", candidate, readErr))
			continue
		}
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			errs = append(errs, fmt.Sprintf("model=%s status=%d body=%s", candidate, resp.StatusCode, string(bytes.TrimSpace(body))))
			continue
		}
		var policy dashScopeUploadPolicyResponse
		if err := json.Unmarshal(body, &policy); err != nil {
			errs = append(errs, fmt.Sprintf("model=%s decode_err=%v", candidate, err))
			continue
		}
		return policy, nil
	}
	if len(errs) == 0 {
		return dashScopeUploadPolicyResponse{}, errors.New("dashscope upload policy failed: no upload policy candidates")
	}
	return dashScopeUploadPolicyResponse{}, fmt.Errorf("dashscope upload policy failed: %s", strings.Join(errs, " | "))
}

func (p *DashScopeVideoProvider) uploadPolicyURL(model string) (string, error) {
	base := strings.TrimSpace(p.Endpoint)
	if base == "" {
		return "", ErrDashScopeVideoEndpointRequired
	}
	u, err := url.Parse(base)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") {
		return "", ErrDashScopeVideoEndpointRequired
	}
	policyPath := strings.TrimSpace(p.UploadPolicyPath)
	if policyPath == "" {
		policyPath = "/uploads"
	}
	u.Path = path.Join(u.Path, policyPath)
	query := u.Query()
	query.Set("action", "getPolicy")
	resolvedModel := strings.TrimSpace(model)
	if resolvedModel == "" {
		resolvedModel = strings.TrimSpace(p.UploadCategory)
	}
	if resolvedModel == "" {
		resolvedModel = "video_generation"
	}
	query.Set("model", resolvedModel)
	u.RawQuery = query.Encode()
	return u.String(), nil
}

func (p *DashScopeVideoProvider) uploadPolicyModels(model string) []string {
	candidates := make([]string, 0, 5)
	appendCandidate := func(value string) {
		value = strings.TrimSpace(value)
		if value == "" || slices.Contains(candidates, value) {
			return
		}
		candidates = append(candidates, value)
	}
	appendCandidate(model)
	if mapped, ok := p.ModelKeyMapping[strings.TrimSpace(model)]; ok {
		appendCandidate(mapped)
	}
	appendCandidate("wan2.6-i2v-flash")
	if mapped, ok := p.ModelKeyMapping["wan2.6-i2v-flash"]; ok {
		appendCandidate(mapped)
	}
	appendCandidate(p.UploadCategory)
	if len(candidates) == 0 {
		candidates = append(candidates, "video_generation")
	}
	return candidates
}

func (p *DashScopeVideoProvider) buildRequestBody(req models.VideoClipRequest) map[string]interface{} {
	resolution := strings.TrimSpace(req.Resolution)
	if resolution == "" {
		resolution = "720P"
	}
	duration := int(req.DurationSec)
	if duration <= 0 {
		duration = 10
	}
	shotType := strings.TrimSpace(req.ShotType)
	if shotType == "" {
		shotType = "multi"
	}
	audioURL := strings.TrimSpace(req.AudioURL)
	audioEnabled := req.AudioEnabled || audioURL != ""
	input := map[string]interface{}{
		"prompt": strings.TrimSpace(req.Prompt),
	}
	if v := strings.TrimSpace(req.ReferenceImageURL); v != "" {
		input["img_url"] = v
	}
	if audioEnabled && audioURL != "" {
		input["audio_url"] = audioURL
	}
	parameters := map[string]interface{}{
		"resolution":    resolution,
		"prompt_extend": req.PromptExtend,
		"duration":      duration,
		"audio":         audioEnabled,
		"shot_type":     shotType,
	}
	if v := strings.TrimSpace(req.NegativePrompt); v != "" {
		parameters["negative_prompt"] = v
	}
	if v := strings.TrimSpace(req.CameraMotion); v != "" {
		parameters["camera_motion"] = v
	}
	return map[string]interface{}{
		"model":      p.resolveModel(req),
		"input":      input,
		"parameters": parameters,
	}
}

func (p *DashScopeVideoProvider) uploadToOSS(ctx context.Context, host string, objectKey string, req models.VideoReferenceUploadRequest, policy dashScopeUploadPolicyResponse) error {
	host = strings.TrimSpace(host)
	if host == "" {
		return errors.New("dashscope oss upload failed: empty host in upload policy response")
	}
	parsed, err := url.Parse(host)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return fmt.Errorf("dashscope oss upload failed: invalid host %q", host)
	}
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	_ = writer.WriteField("key", objectKey)
	_ = writer.WriteField("OSSAccessKeyId", strings.TrimSpace(policy.Output.OssAccessKeyID))
	_ = writer.WriteField("policy", strings.TrimSpace(policy.Output.Policy))
	_ = writer.WriteField("signature", strings.TrimSpace(policy.Output.Signature))
	_ = writer.WriteField("success_action_status", "200")
	part, err := writer.CreateFormFile("file", normalizedUploadFileName(req.FileName, req.ContentType))
	if err != nil {
		return err
	}
	if _, err := part.Write(req.Content); err != nil {
		return err
	}
	if err := writer.Close(); err != nil {
		return err
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, host, body)
	if err != nil {
		return err
	}
	httpReq.Header.Set("Content-Type", writer.FormDataContentType())
	resp, err := p.httpClient().Do(httpReq)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("dashscope oss upload failed: status=%d body=%s", resp.StatusCode, string(bytes.TrimSpace(respBody)))
	}
	return nil
}

type dashScopeUploadPolicyResponse struct {
	Output struct {
		Policy         string `json:"policy"`
		Signature      string `json:"signature"`
		OssAccessKeyID string `json:"oss_access_key_id"`
		Dir            string `json:"dir"`
		Host           string `json:"host"`
		UUID           string `json:"uuid"`
	} `json:"output"`
}

func buildUploadObjectKey(dir string, uuid string, fileName string, contentType string) string {
	dir = strings.Trim(strings.TrimSpace(dir), "/")
	uuid = strings.TrimSpace(uuid)
	if uuid == "" {
		uuid = strings.TrimSuffix(strings.TrimSpace(fileName), filepath.Ext(strings.TrimSpace(fileName)))
	}
	if uuid == "" {
		uuid = fmt.Sprintf("upload_%d", time.Now().UnixNano())
	}
	ext := strings.ToLower(strings.TrimSpace(filepath.Ext(strings.TrimSpace(fileName))))
	if ext == "" {
		ext = inferUploadExtension(contentType)
	}
	name := uuid + ext
	if dir == "" {
		return name
	}
	return dir + "/" + name
}

func normalizedUploadFileName(fileName string, contentType string) string {
	name := strings.TrimSpace(fileName)
	if name == "" {
		name = "reference" + inferUploadExtension(contentType)
	}
	if filepath.Ext(name) == "" {
		name += inferUploadExtension(contentType)
	}
	return name
}

func inferUploadExtension(contentType string) string {
	mediaType, _, err := mime.ParseMediaType(strings.TrimSpace(contentType))
	if err == nil {
		switch strings.ToLower(mediaType) {
		case "image/jpeg":
			return ".jpg"
		case "image/webp":
			return ".webp"
		case "image/gif":
			return ".gif"
		case "image/png":
			return ".png"
		}
	}
	return ".png"
}

func bucketFromOSSHost(host string) string {
	parsed, err := url.Parse(strings.TrimSpace(host))
	if err != nil {
		return "dashscope-instant"
	}
	hostname := strings.TrimSpace(parsed.Hostname())
	if hostname == "" {
		return "dashscope-instant"
	}
	if net.ParseIP(hostname) != nil {
		return hostname
	}
	parts := strings.Split(hostname, ".")
	if len(parts) == 0 || strings.TrimSpace(parts[0]) == "" {
		return "dashscope-instant"
	}
	return strings.TrimSpace(parts[0])
}

func requiresOSSResourceResolve(values ...string) bool {
	for _, value := range values {
		if strings.HasPrefix(strings.ToLower(strings.TrimSpace(value)), "oss://") {
			return true
		}
	}
	return false
}

func (p *DashScopeVideoProvider) resolveModel(req models.VideoClipRequest) string {
	model := strings.TrimSpace(req.Model)
	if model == "" {
		model = "wan2.6-i2v-flash"
	}
	if mapped, ok := p.ModelKeyMapping[model]; ok && strings.TrimSpace(mapped) != "" {
		return strings.TrimSpace(mapped)
	}
	return model
}

func (p *DashScopeVideoProvider) parseTaskResponse(data []byte) (*models.VideoProviderTask, error) {
	var raw map[string]interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, err
	}
	taskID := firstNonEmpty(
		firstString(raw, "task_id"),
		nestedFirstString(raw, []string{"output", "task_id"}),
	)
	if taskID == "" {
		return nil, errors.New("dashscope video missing task id")
	}
	providerStatus := firstNonEmpty(
		firstString(raw, "task_status"),
		nestedFirstString(raw, []string{"output", "task_status"}),
		nestedFirstString(raw, []string{"output", "status"}),
	)
	resultURL := firstNonEmpty(
		nestedFirstString(raw, []string{"output", "video_url"}),
		nestedFirstString(raw, []string{"output", "video", "url"}),
		nestedFirstString(raw, []string{"output", "results", "0", "url"}),
	)
	errMessage := firstNonEmpty(
		firstString(raw, "message"),
		nestedFirstString(raw, []string{"output", "message"}),
		nestedFirstString(raw, []string{"output", "error_message"}),
	)
	return &models.VideoProviderTask{
		TaskID:         taskID,
		Status:         normalizeDashScopeTaskStatus(providerStatus),
		ResultURL:      resultURL,
		ErrorMessage:   errMessage,
		ProviderStatus: providerStatus,
		Raw:            raw,
	}, nil
}

func normalizeDashScopeTaskStatus(status string) string {
	status = strings.ToUpper(strings.TrimSpace(status))
	switch status {
	case "SUCCEEDED", "SUCCESS", "COMPLETED":
		return "completed"
	case "FAILED", "ERROR", "CANCELED", "CANCELLED":
		return "failed"
	case "RUNNING", "PENDING", "SUBMITTED", "QUEUED":
		return "running"
	default:
		return strings.ToLower(status)
	}
}

func firstString(raw map[string]interface{}, key string) string {
	v, ok := raw[key]
	if !ok {
		return ""
	}
	s, _ := v.(string)
	return strings.TrimSpace(s)
}

func nestedFirstString(raw map[string]interface{}, items []string) string {
	var cur interface{} = raw
	for _, item := range items {
		switch node := cur.(type) {
		case map[string]interface{}:
			cur = node[item]
		case []interface{}:
			if item != "0" || len(node) == 0 {
				return ""
			}
			cur = node[0]
		default:
			return ""
		}
	}
	s, _ := cur.(string)
	return strings.TrimSpace(s)
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}
