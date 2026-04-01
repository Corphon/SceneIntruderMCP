// internal/services/video_service.go
package services

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"io"
	"mime"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/Corphon/SceneIntruderMCP/internal/models"
)

var (
	ErrVideoRepositoryNotReady        = errors.New("video repository not initialized")
	ErrVideoServiceNotReady           = errors.New("video service dependencies not ready")
	ErrVideoTimelineNotFound          = errors.New("video timeline not found")
	ErrVideoMetaNotFound              = errors.New("video meta not found")
	ErrVideoSourceNotReady            = errors.New("comic source assets are not ready for video")
	ErrVideoFrameNotFound             = errors.New("video frame not found")
	ErrVideoReferenceImageURLRequired = errors.New("video reference image url is required")
	ErrVideoPublicBaseURLInvalid      = errors.New("video public_base_url is invalid")
)

const (
	VideoJobTypeTimelineBuild = "video_timeline_build"
	VideoJobTypeClipGenerate  = "video_clip_generate"
	VideoJobTypeRenderCompose = "video_render_compose"
	VideoJobTypeExport        = "video_export"
	videoStageTotal           = 4
)

// VideoProvider is the future provider abstraction for DashScope and other video backends.
type VideoProvider interface {
	SubmitClipTask(ctx context.Context, req models.VideoClipRequest) (*models.VideoProviderTask, error)
	PollTask(ctx context.Context, taskID string) (*models.VideoProviderTask, error)
}

type VideoReferenceUploader interface {
	UploadReferenceImage(ctx context.Context, req models.VideoReferenceUploadRequest) (*models.VideoReferenceUploadResult, error)
}

// VideoService orchestrates the independent video module derived from comic outputs.
type VideoService struct {
	Repo      *VideoRepository
	ComicRepo *ComicRepository
	JobQueue  *JobQueue
	Progress  *ProgressService
	Provider  VideoProvider

	DefaultProvider        string
	DefaultModel           string
	PollInterval           time.Duration
	MaxPollAttempts        int
	MaxClipRetries         int
	ClipCacheEnabled       bool
	FallbackComposeEnabled bool
	PublicBaseURL          string
	DownloadTimeout        time.Duration
	FFmpegPath             string
	CommandRunner          func(ctx context.Context, name string, args ...string) *exec.Cmd
}

func NewVideoService(repo *VideoRepository, comicRepo *ComicRepository, jobQueue *JobQueue, progress *ProgressService, provider VideoProvider) *VideoService {
	return &VideoService{
		Repo:                   repo,
		ComicRepo:              comicRepo,
		JobQueue:               jobQueue,
		Progress:               progress,
		Provider:               provider,
		DefaultProvider:        "dashscope",
		DefaultModel:           "wan2.6-i2v-flash",
		PollInterval:           2 * time.Second,
		MaxPollAttempts:        90,
		MaxClipRetries:         1,
		ClipCacheEnabled:       true,
		FallbackComposeEnabled: true,
		DownloadTimeout:        60 * time.Second,
		FFmpegPath:             "ffmpeg",
	}
}

func (s *VideoService) ensureReady() error {
	if s.Repo == nil || s.ComicRepo == nil || s.JobQueue == nil || s.Progress == nil {
		return ErrVideoServiceNotReady
	}
	return nil
}

func sanitizeVideoConfig(cfg models.VideoConfig, defaultProvider string, defaultModel string) models.VideoConfig {
	cfg.ComicSnapshotID = strings.TrimSpace(cfg.ComicSnapshotID)
	if cfg.ComicSnapshotID == "" {
		cfg.ComicSnapshotID = "comic_current"
	}
	cfg.Provider = strings.TrimSpace(cfg.Provider)
	if cfg.Provider == "" {
		cfg.Provider = strings.TrimSpace(defaultProvider)
		if cfg.Provider == "" {
			cfg.Provider = "dashscope"
		}
	}
	cfg.Model = strings.TrimSpace(cfg.Model)
	if cfg.Model == "" {
		cfg.Model = strings.TrimSpace(defaultModel)
		if cfg.Model == "" {
			cfg.Model = "wan2.6-i2v-flash"
		}
	}
	if cfg.TargetDurationSec <= 0 && cfg.Duration <= 0 {
		cfg.TargetDurationSec = 16
	}
	if cfg.FPS <= 0 {
		cfg.FPS = 24
	}
	cfg.Resolution = strings.TrimSpace(cfg.Resolution)
	if cfg.Resolution == "" {
		cfg.Resolution = "720P"
	}
	cfg.ShotType = strings.TrimSpace(cfg.ShotType)
	if cfg.ShotType == "" {
		cfg.ShotType = "multi"
	}
	cfg.AudioURL = strings.TrimSpace(cfg.AudioURL)
	if cfg.AudioURL != "" {
		cfg.AudioEnabled = true
	}
	cfg.ImageURL = strings.TrimSpace(cfg.ImageURL)
	cfg.PromptOptions = sanitizeVideoPromptOptions(cfg.PromptOptions)
	return cfg
}

func fallbackTimelineVideoPrompt(imagePrompt string, frameDescription string) string {
	return buildEnhancedVideoPrompt(buildFallbackVideoPromptBase(imagePrompt, frameDescription), frameDescription, models.VideoPromptOptions{})
}

func fallbackTimelineCameraMotion(frameDescription string) string {
	desc := strings.ToLower(strings.TrimSpace(frameDescription))
	switch {
	case strings.Contains(desc, "追"), strings.Contains(desc, "run"), strings.Contains(desc, "chase"), strings.Contains(desc, "follow"):
		return "handheld"
	case strings.Contains(desc, "pan"), strings.Contains(desc, "横移"), strings.Contains(desc, "扫"):
		return "pan_right"
	case strings.Contains(desc, "rise"), strings.Contains(desc, "up"), strings.Contains(desc, "抬头"), strings.Contains(desc, "仰"):
		return "tilt_up"
	case strings.Contains(desc, "down"), strings.Contains(desc, "俯"), strings.Contains(desc, "低头"):
		return "tilt_down"
	case strings.Contains(desc, "close"), strings.Contains(desc, "靠近"), strings.Contains(desc, "推进"):
		return "push_in"
	case strings.Contains(desc, "远"), strings.Contains(desc, "拉远"), strings.Contains(desc, "wide"):
		return "pull_out"
	default:
		return "static"
	}
}

type videoSourceFrame struct {
	FrameID        string `json:"frame_id"`
	Order          int    `json:"order"`
	Prompt         string `json:"prompt"`
	VideoPrompt    string `json:"video_prompt,omitempty"`
	NegativePrompt string `json:"negative_prompt,omitempty"`
	Style          string `json:"style,omitempty"`
	Model          string `json:"model,omitempty"`
	CameraMotion   string `json:"camera_motion,omitempty"`
	Narration      string `json:"narration,omitempty"`
	MusicCue       string `json:"music_cue,omitempty"`
	SFX            string `json:"sfx,omitempty"`
	ImageHash      string `json:"image_hash,omitempty"`
}

func (s *VideoService) EnsureVideoLayout(sceneID string) error {
	if s.Repo == nil {
		return ErrVideoRepositoryNotReady
	}
	return s.Repo.EnsureVideoLayout(sceneID)
}

func (s *VideoService) BuildTimeline(sceneID string, cfg models.VideoConfig) (*models.VideoTimeline, error) {
	if err := s.ensureReady(); err != nil {
		return nil, err
	}
	cfg = sanitizeVideoConfig(cfg, s.DefaultProvider, s.DefaultModel)
	if err := s.Repo.EnsureVideoLayout(sceneID); err != nil {
		return nil, err
	}

	analysis, err := s.ComicRepo.LoadAnalysis(sceneID)
	if err != nil {
		return nil, fmt.Errorf("load comic analysis: %w", err)
	}
	if len(analysis.Frames) == 0 {
		return nil, ErrVideoSourceNotReady
	}

	sourceFrames := make([]videoSourceFrame, 0, len(analysis.Frames))
	clips := make([]models.VideoTimelineClip, 0, len(analysis.Frames))
	totalFrames := len(analysis.Frames)
	targetDurationSec := cfg.TargetDurationSec
	clipDuration := 0.0
	if cfg.Duration > 0 {
		clipDuration = float64(cfg.Duration)
		targetDurationSec = cfg.Duration * totalFrames
	}
	if clipDuration <= 0 {
		clipDuration = float64(targetDurationSec) / float64(totalFrames)
	}
	if clipDuration <= 0 {
		clipDuration = 4
	}
	if targetDurationSec <= 0 {
		targetDurationSec = int(clipDuration * float64(totalFrames))
		if targetDurationSec <= 0 {
			targetDurationSec = totalFrames * 4
		}
	}
	ordered := append([]models.ComicFramePlan(nil), analysis.Frames...)
	sort.SliceStable(ordered, func(i, j int) bool { return ordered[i].Order < ordered[j].Order })

	for idx, frame := range ordered {
		frameID := strings.TrimSpace(frame.ID)
		if frameID == "" {
			frameID = fmt.Sprintf("frame_%d", idx+1)
		}
		prompt, err := s.ComicRepo.LoadPrompt(sceneID, frameID)
		if err != nil {
			return nil, fmt.Errorf("load prompt %s: %w", frameID, err)
		}
		img, err := s.ComicRepo.LoadFrameImage(sceneID, frameID)
		if err != nil {
			return nil, fmt.Errorf("load image %s: %w", frameID, err)
		}
		imageHash := sha256Hex(img)
		sourceModel := strings.TrimSpace(prompt.Model)
		model := strings.TrimSpace(cfg.Model)
		if model == "" {
			model = sourceModel
		}
		frameDescription := strings.TrimSpace(frame.Description)
		promptOptions := cfg.PromptOptions
		promptBase := strings.TrimSpace(prompt.VideoPrompt)
		if promptBase == "" {
			promptBase = buildFallbackVideoPromptBase(prompt.Prompt, frameDescription)
		}
		videoPrompt := buildEnhancedVideoPrompt(promptBase, frameDescription, promptOptions)
		cameraMotion := strings.TrimSpace(prompt.CameraMotion)
		if cameraMotion == "" {
			cameraMotion = fallbackTimelineCameraMotion(frameDescription)
		}
		relImagePath := filepath.ToSlash(filepath.Join("scene_"+sceneID, "images", frameID+".png"))
		referenceImageURL, err := s.resolveReferenceImageURL(context.Background(), sceneID, frameID, cfg.Provider, model, cfg.ImageURL, img)
		if err != nil {
			return nil, err
		}
		clip := models.VideoTimelineClip{
			FrameID:            frameID,
			Order:              frame.Order,
			PromptBase:         promptBase,
			Prompt:             videoPrompt,
			NegativePrompt:     strings.TrimSpace(prompt.NegativePrompt),
			Style:              strings.TrimSpace(prompt.Style),
			Model:              model,
			ImagePath:          relImagePath,
			ImageHash:          imageHash,
			ReferenceImageURL:  referenceImageURL,
			DurationSec:        clipDuration,
			Transition:         "cut",
			CameraMotion:       cameraMotion,
			AnimationIntensity: "low",
			Narration:          strings.TrimSpace(prompt.Narration),
			MusicCue:           strings.TrimSpace(prompt.MusicCue),
			SFX:                strings.TrimSpace(prompt.SFX),
			PromptOptions:      promptOptions,
		}
		clips = append(clips, clip)
		sourceFrames = append(sourceFrames, videoSourceFrame{
			FrameID:        frameID,
			Order:          frame.Order,
			Prompt:         strings.TrimSpace(prompt.Prompt),
			VideoPrompt:    clip.Prompt,
			NegativePrompt: clip.NegativePrompt,
			Style:          clip.Style,
			Model:          sourceModel,
			CameraMotion:   clip.CameraMotion,
			Narration:      clip.Narration,
			MusicCue:       clip.MusicCue,
			SFX:            clip.SFX,
			ImageHash:      clip.ImageHash,
		})
	}

	sourceVersionHash, err := hashVideoSource(sourceFrames)
	if err != nil {
		return nil, err
	}
	now := time.Now()
	videoVersion := fmt.Sprintf("video_%d", now.UnixNano())
	timeline := &models.VideoTimeline{
		SceneID:      sceneID,
		VideoVersion: videoVersion,
		ComicSnapshot: models.ComicSnapshotRef{
			SceneID:           sceneID,
			SourceComicID:     sceneID,
			ComicSnapshotID:   cfg.ComicSnapshotID,
			SourceVersionHash: sourceVersionHash,
			CreatedAt:         now,
		},
		Provider:          cfg.Provider,
		Model:             cfg.Model,
		TargetDurationSec: targetDurationSec,
		FPS:               cfg.FPS,
		Resolution:        cfg.Resolution,
		AudioEnabled:      cfg.AudioEnabled,
		ShotType:          cfg.ShotType,
		PromptExtend:      cfg.PromptExtend,
		AudioURL:          cfg.AudioURL,
		PromptOptions:     cfg.PromptOptions,
		Clips:             clips,
		GeneratedAt:       now,
		UpdatedAt:         now,
	}
	if err := s.Repo.SaveTimeline(sceneID, timeline); err != nil {
		return nil, err
	}

	meta, _ := s.Repo.LoadMeta(sceneID)
	if meta == nil {
		meta = &models.VideoMeta{SceneID: sceneID, CreatedAt: now}
	}
	meta.CurrentVideoVersion = videoVersion
	meta.ComicSnapshotID = cfg.ComicSnapshotID
	meta.SourceVersionHash = sourceVersionHash
	meta.Provider = cfg.Provider
	meta.Model = cfg.Model
	meta.TimelineStatus = "built"
	if meta.GenerationStatus == "" {
		meta.GenerationStatus = "idle"
	}
	if meta.RenderStatus == "" {
		meta.RenderStatus = "idle"
	}
	meta.ClipCount = len(clips)
	meta.CompletedClipCount = 0
	meta.LastError = ""
	meta.UpdatedAt = now
	if err := s.Repo.SaveMeta(sceneID, meta); err != nil {
		return nil, err
	}

	return timeline, nil
}

func (s *VideoService) LoadTimeline(sceneID string) (*models.VideoTimeline, error) {
	if s.Repo == nil {
		return nil, ErrVideoRepositoryNotReady
	}
	timeline, err := s.Repo.LoadTimeline(sceneID)
	if err != nil {
		if isLikelyNotExist(err) {
			return nil, ErrVideoTimelineNotFound
		}
		return nil, err
	}
	return timeline, nil
}

func (s *VideoService) LoadOverview(sceneID string) (*models.VideoOverview, error) {
	if err := s.ensureReady(); err != nil {
		return nil, err
	}
	timeline, err := s.LoadTimeline(sceneID)
	if err != nil {
		return nil, err
	}
	meta, err := s.Repo.LoadMeta(sceneID)
	if err != nil {
		if isLikelyNotExist(err) {
			return nil, ErrVideoMetaNotFound
		}
		return nil, err
	}
	currentHash, hashErr := s.ComputeCurrentComicSourceHash(sceneID)
	stale := false
	if hashErr != nil || currentHash == "" {
		stale = true
	} else if meta.SourceVersionHash != "" && meta.SourceVersionHash != currentHash {
		stale = true
	}
	clips := make([]models.VideoClipOverviewItem, 0, len(timeline.Clips))
	for _, clip := range timeline.Clips {
		item := models.VideoClipOverviewItem{
			FrameID:           clip.FrameID,
			Order:             clip.Order,
			Prompt:            clip.Prompt,
			DurationSec:       clip.DurationSec,
			ImagePath:         clip.ImagePath,
			ReferenceImageURL: clip.ReferenceImageURL,
		}
		result, loadErr := s.LoadClipResult(sceneID, clip.FrameID)
		if loadErr != nil {
			if !isLikelyNotExist(loadErr) {
				return nil, loadErr
			}
			clips = append(clips, item)
			continue
		}
		item.Status = result.Status
		item.ProviderStatus = result.ProviderStatus
		item.AttemptCount = result.AttemptCount
		item.VideoURL = result.VideoURL
		item.LocalPath = result.LocalPath
		item.ErrorMessage = result.ErrorMessage
		item.FromCache = result.FromCache
		item.GeneratedAt = result.GeneratedAt
		clips = append(clips, item)
	}
	recovery := s.buildRecoveryInfo(timeline, meta, clips)
	return &models.VideoOverview{
		SceneID:                   sceneID,
		Timeline:                  timeline,
		Meta:                      meta,
		Clips:                     clips,
		Recovery:                  recovery,
		ComicSnapshotID:           timeline.ComicSnapshot.ComicSnapshotID,
		VideoVersion:              timeline.VideoVersion,
		IsStaleAgainstLatestComic: stale,
	}, nil
}

func (s *VideoService) buildRecoveryInfo(timeline *models.VideoTimeline, meta *models.VideoMeta, clips []models.VideoClipOverviewItem) *models.VideoRecoveryInfo {
	if timeline == nil {
		return nil
	}
	info := &models.VideoRecoveryInfo{
		Status:            "idle",
		PendingFrameIDs:   make([]string, 0, len(timeline.Clips)),
		FailedFrameIDs:    make([]string, 0, len(timeline.Clips)),
		CompletedFrameIDs: make([]string, 0, len(timeline.Clips)),
	}
	clipByFrame := make(map[string]models.VideoClipOverviewItem, len(clips))
	for _, clip := range clips {
		clipByFrame[clip.FrameID] = clip
	}
	for _, timelineClip := range timeline.Clips {
		frameID := strings.TrimSpace(timelineClip.FrameID)
		clip, ok := clipByFrame[frameID]
		if !ok {
			info.PendingFrameIDs = append(info.PendingFrameIDs, frameID)
			continue
		}
		switch strings.ToLower(strings.TrimSpace(clip.Status)) {
		case "completed":
			info.CompletedFrameIDs = append(info.CompletedFrameIDs, frameID)
		case "failed":
			info.FailedFrameIDs = append(info.FailedFrameIDs, frameID)
			info.PendingFrameIDs = append(info.PendingFrameIDs, frameID)
		default:
			info.PendingFrameIDs = append(info.PendingFrameIDs, frameID)
		}
	}
	if meta == nil {
		if len(info.PendingFrameIDs) == 0 {
			info.Status = "completed"
		}
		return info
	}
	lastTaskID := strings.TrimSpace(meta.LastTaskID)
	if lastTaskID != "" && s.Progress != nil {
		if tracker, ok := s.Progress.GetTracker(lastTaskID); ok && tracker != nil {
			snapshot := tracker.Snapshot()
			info.ActiveTaskID = snapshot.TaskID
			info.ActiveTaskStatus = strings.TrimSpace(snapshot.Status)
			info.LastProgress = snapshot.Progress
			info.LastMessage = strings.TrimSpace(snapshot.Message)
		}
	}
	generationStatus := strings.ToLower(strings.TrimSpace(meta.GenerationStatus))
	switch generationStatus {
	case "running":
		if info.ActiveTaskID != "" && info.ActiveTaskStatus == "running" {
			info.Status = "running"
			return info
		}
		info.Status = "interrupted"
		info.CanResume = len(info.PendingFrameIDs) > 0
		if info.CanResume {
			info.ResumeFrom = "interrupted_task"
		}
	case "failed":
		info.Status = "failed"
		info.CanResume = len(info.PendingFrameIDs) > 0
		if info.CanResume {
			info.Status = "resumable"
			if len(info.FailedFrameIDs) > 0 {
				info.ResumeFrom = "failed_clips"
			} else {
				info.ResumeFrom = "incomplete_clips"
			}
		}
	case "completed_degraded":
		info.Status = "completed_degraded"
		info.CanResume = len(info.FailedFrameIDs) > 0
		if info.CanResume {
			info.Status = "resumable"
			info.ResumeFrom = "failed_clips"
		}
	case "completed":
		info.Status = "completed"
	case "idle", "", "built":
		if len(info.CompletedFrameIDs) == len(timeline.Clips) && len(timeline.Clips) > 0 {
			info.Status = "completed"
		} else {
			info.Status = "idle"
		}
	default:
		info.Status = generationStatus
	}
	return info
}

func (s *VideoService) ComputeCurrentComicSourceHash(sceneID string) (string, error) {
	if s.ComicRepo == nil {
		return "", ErrComicRepositoryNotReady
	}
	promptOptionsByFrame := map[string]models.VideoPromptOptions{}
	if timeline, err := s.LoadTimeline(sceneID); err == nil && timeline != nil {
		for _, clip := range timeline.Clips {
			promptOptionsByFrame[strings.TrimSpace(clip.FrameID)] = sanitizeVideoPromptOptions(clip.PromptOptions)
		}
	}
	analysis, err := s.ComicRepo.LoadAnalysis(sceneID)
	if err != nil {
		return "", err
	}
	if len(analysis.Frames) == 0 {
		return "", ErrVideoSourceNotReady
	}
	ordered := append([]models.ComicFramePlan(nil), analysis.Frames...)
	sort.SliceStable(ordered, func(i, j int) bool { return ordered[i].Order < ordered[j].Order })
	sourceFrames := make([]videoSourceFrame, 0, len(ordered))
	for idx, frame := range ordered {
		frameID := strings.TrimSpace(frame.ID)
		if frameID == "" {
			frameID = fmt.Sprintf("frame_%d", idx+1)
		}
		prompt, err := s.ComicRepo.LoadPrompt(sceneID, frameID)
		if err != nil {
			return "", err
		}
		img, err := s.ComicRepo.LoadFrameImage(sceneID, frameID)
		if err != nil {
			return "", err
		}
		frameDescription := strings.TrimSpace(frame.Description)
		videoPrompt := strings.TrimSpace(prompt.VideoPrompt)
		if videoPrompt == "" {
			videoPrompt = buildFallbackVideoPromptBase(prompt.Prompt, frameDescription)
		}
		videoPrompt = buildEnhancedVideoPrompt(videoPrompt, frameDescription, promptOptionsByFrame[frameID])
		cameraMotion := strings.TrimSpace(prompt.CameraMotion)
		if cameraMotion == "" {
			cameraMotion = fallbackTimelineCameraMotion(frameDescription)
		}
		sourceFrames = append(sourceFrames, videoSourceFrame{
			FrameID:        frameID,
			Order:          frame.Order,
			Prompt:         strings.TrimSpace(prompt.Prompt),
			VideoPrompt:    videoPrompt,
			NegativePrompt: strings.TrimSpace(prompt.NegativePrompt),
			Style:          strings.TrimSpace(prompt.Style),
			Model:          strings.TrimSpace(prompt.Model),
			CameraMotion:   cameraMotion,
			Narration:      strings.TrimSpace(prompt.Narration),
			MusicCue:       strings.TrimSpace(prompt.MusicCue),
			SFX:            strings.TrimSpace(prompt.SFX),
			ImageHash:      sha256Hex(img),
		})
	}
	return hashVideoSource(sourceFrames)
}

func (s *VideoService) GenerateVideoAsync(ctx context.Context, sceneID string, cfg models.VideoConfig) (string, error) {
	if err := s.ensureReady(); err != nil {
		return "", err
	}
	cfg = sanitizeVideoConfig(cfg, s.DefaultProvider, s.DefaultModel)
	timeline, err := s.BuildTimeline(sceneID, cfg)
	if err != nil {
		return "", err
	}
	if err := s.validateTimelineForGeneration(timeline); err != nil {
		return "", err
	}
	taskID := fmt.Sprintf("video_generate_%s_%d", sceneID, time.Now().UnixNano())
	tracker := s.Progress.CreateTracker(taskID)
	if err := s.JobQueue.Submit(taskID, func(jobCtx context.Context) error {
		tracker.EmitProgressEventWithMeta(5, "validating video snapshot", "video_validate", "", &ProgressEventMeta{
			Phase:      "video_validate",
			StageIndex: 1,
			StageTotal: videoStageTotal,
		})
		if err := s.updateMetaRunning(sceneID, taskID, timeline); err != nil {
			tracker.Fail(err.Error())
			return err
		}
		tracker.EmitProgressEventWithMeta(15, "timeline built", VideoJobTypeTimelineBuild, "", &ProgressEventMeta{
			Phase:        VideoJobTypeTimelineBuild,
			VideoVersion: timeline.VideoVersion,
			StageIndex:   2,
			StageTotal:   videoStageTotal,
		})
		clipTotal := len(timeline.Clips)
		var lastResult *models.VideoClipResult
		clipResults := make(map[string]*models.VideoClipResult, clipTotal)
		degraded := false
		for i, clip := range timeline.Clips {
			select {
			case <-jobCtx.Done():
				tracker.Fail("视频任务已取消")
				_ = s.updateMetaFailure(sceneID, taskID, nil, jobCtx.Err())
				return jobCtx.Err()
			default:
			}
			progress := 20
			if clipTotal > 0 {
				progress = 20 + int(float64(i+1)/float64(clipTotal)*60.0)
			}
			result, err := s.generateClip(jobCtx, tracker, timeline, clip, progress, fmt.Sprintf("Generating clip %d/%d", i+1, clipTotal))
			if err != nil {
				if result != nil {
					_ = s.Repo.SaveClipResult(sceneID, timeline.VideoVersion, clip.FrameID, result)
					clipResults[clip.FrameID] = result
					lastResult = result
				}
				if s.FallbackComposeEnabled {
					degraded = true
					if tracker != nil {
						providerTaskID := ""
						providerStatus := "fallback_pending"
						if result != nil {
							providerTaskID = result.ProviderTaskID
							if strings.TrimSpace(result.ProviderStatus) != "" {
								providerStatus = result.ProviderStatus
							}
						}
						tracker.EmitProgressEventWithMeta(progress, fmt.Sprintf("Clip %s failed, fallback compose will continue", clip.FrameID), VideoJobTypeClipGenerate, clip.FrameID, &ProgressEventMeta{
							Phase:          VideoJobTypeClipGenerate,
							VideoVersion:   timeline.VideoVersion,
							StageIndex:     3,
							StageTotal:     videoStageTotal,
							ProviderTaskID: providerTaskID,
							ProviderStatus: providerStatus,
						})
					}
					if err := s.updateMetaClipProgress(sceneID, taskID, timeline, i+1, result); err != nil {
						tracker.Fail(err.Error())
						return err
					}
					continue
				}
				tracker.Fail(err.Error())
				_ = s.updateMetaFailure(sceneID, taskID, result, err)
				return err
			}
			lastResult = result
			clipResults[clip.FrameID] = result
			if err := s.Repo.SaveClipResult(sceneID, timeline.VideoVersion, clip.FrameID, result); err != nil {
				tracker.Fail(err.Error())
				_ = s.updateMetaFailure(sceneID, taskID, result, err)
				return err
			}
			if err := s.updateMetaClipProgress(sceneID, taskID, timeline, i+1, result); err != nil {
				tracker.Fail(err.Error())
				return err
			}
		}
		tracker.EmitProgressEventWithMeta(95, "composing render", VideoJobTypeRenderCompose, "", &ProgressEventMeta{
			Phase:          VideoJobTypeRenderCompose,
			VideoVersion:   timeline.VideoVersion,
			StageIndex:     4,
			StageTotal:     videoStageTotal,
			ProviderStatus: map[bool]string{true: "degraded", false: "completed"}[degraded],
		})
		renderArtifactPath := ""
		renderArtifactType := ""
		artifactPath, artifactType, err := s.composeRenderArtifact(sceneID, timeline, clipResults)
		if err != nil {
			tracker.Fail(err.Error())
			_ = s.updateMetaFailure(sceneID, taskID, lastResult, err)
			return err
		}
		renderArtifactPath = artifactPath
		renderArtifactType = artifactType
		if err := s.updateMetaCompleted(sceneID, taskID, timeline, lastResult, renderArtifactPath, renderArtifactType, degraded); err != nil {
			tracker.Fail(err.Error())
			return err
		}
		if degraded {
			tracker.Complete("视频任务已完成（degraded fallback compose）")
			return nil
		}
		if renderArtifactType == "mp4_stitched_video" {
			tracker.Complete("视频任务已完成（已合成为单文件 MP4）")
			return nil
		}
		if renderArtifactType == "html_stitched_video" {
			tracker.Complete("视频任务已完成（已合并为单一连续播放入口）")
			return nil
		}
		tracker.Complete("视频任务已完成（当前为占位编排）")
		return nil
	}); err != nil {
		return "", err
	}
	return taskID, nil
}

func (s *VideoService) RegenerateClipAsync(ctx context.Context, sceneID string, frameID string, cfg models.VideoConfig) (string, error) {
	if err := s.ensureReady(); err != nil {
		return "", err
	}
	cfg = sanitizeVideoConfig(cfg, s.DefaultProvider, s.DefaultModel)
	if err := validatePathSegment(strings.TrimSpace(frameID)); err != nil {
		return "", err
	}
	timeline, err := s.LoadTimeline(sceneID)
	if err != nil {
		return "", err
	}
	clip, err := findTimelineClip(timeline, frameID)
	if err != nil {
		return "", err
	}
	if s.Provider != nil {
		if _, err := s.buildClipRequest(timeline, clip); err != nil {
			return "", err
		}
	}
	if strings.TrimSpace(cfg.ComicSnapshotID) == "" {
		cfg.ComicSnapshotID = timeline.ComicSnapshot.ComicSnapshotID
	}
	taskID := fmt.Sprintf("video_regenerate_%s_%s_%d", sceneID, frameID, time.Now().UnixNano())
	tracker := s.Progress.CreateTracker(taskID)
	if err := s.JobQueue.Submit(taskID, func(jobCtx context.Context) error {
		tracker.EmitProgressEventWithMeta(10, "validating clip regenerate request", "video_validate", frameID, &ProgressEventMeta{
			Phase:        "video_validate",
			VideoVersion: timeline.VideoVersion,
			StageIndex:   1,
			StageTotal:   videoStageTotal,
		})
		if err := s.updateMetaRegenerateRunning(sceneID, taskID, timeline); err != nil {
			tracker.Fail(err.Error())
			return err
		}
		result, err := s.generateClip(jobCtx, tracker, timeline, clip, 55, fmt.Sprintf("Regenerating clip %s", frameID))
		if err != nil {
			if result != nil {
				_ = s.Repo.SaveClipResult(sceneID, timeline.VideoVersion, frameID, result)
			}
			tracker.Fail(err.Error())
			_ = s.updateMetaFailure(sceneID, taskID, result, err)
			return err
		}
		if err := s.Repo.SaveClipResult(sceneID, timeline.VideoVersion, frameID, result); err != nil {
			tracker.Fail(err.Error())
			_ = s.updateMetaFailure(sceneID, taskID, result, err)
			return err
		}
		tracker.EmitProgressEventWithMeta(90, "refreshing video render metadata", VideoJobTypeRenderCompose, frameID, &ProgressEventMeta{
			Phase:          VideoJobTypeRenderCompose,
			VideoVersion:   timeline.VideoVersion,
			StageIndex:     4,
			StageTotal:     videoStageTotal,
			ProviderTaskID: result.ProviderTaskID,
			ProviderStatus: result.ProviderStatus,
		})
		clipResults, err := s.loadTimelineClipResults(sceneID, timeline)
		if err != nil {
			tracker.Fail(err.Error())
			return err
		}
		renderArtifactPath, renderArtifactType, err := s.composeRenderArtifact(sceneID, timeline, clipResults)
		if err != nil {
			tracker.Fail(err.Error())
			return err
		}
		if err := s.updateMetaCompleted(sceneID, taskID, timeline, result, renderArtifactPath, renderArtifactType, false); err != nil {
			tracker.Fail(err.Error())
			return err
		}
		tracker.Complete("视频分镜重生成已完成（当前为占位编排）")
		return nil
	}); err != nil {
		return "", err
	}
	return taskID, nil
}

func (s *VideoService) updateMetaRunning(sceneID string, taskID string, timeline *models.VideoTimeline) error {
	meta, _ := s.Repo.LoadMeta(sceneID)
	if meta == nil {
		meta = &models.VideoMeta{SceneID: sceneID, CreatedAt: time.Now()}
	}
	meta.CurrentVideoVersion = timeline.VideoVersion
	meta.ComicSnapshotID = timeline.ComicSnapshot.ComicSnapshotID
	meta.SourceVersionHash = timeline.ComicSnapshot.SourceVersionHash
	meta.Provider = timeline.Provider
	meta.Model = timeline.Model
	meta.TimelineStatus = "built"
	meta.GenerationStatus = "running"
	meta.RenderStatus = "running"
	meta.ClipCount = len(timeline.Clips)
	meta.CompletedClipCount = 0
	meta.LastTaskID = taskID
	meta.LastProviderTaskID = ""
	meta.LastError = ""
	meta.UpdatedAt = time.Now()
	return s.Repo.SaveMeta(sceneID, meta)
}

func (s *VideoService) updateMetaClipProgress(sceneID string, taskID string, timeline *models.VideoTimeline, completed int, result *models.VideoClipResult) error {
	meta, _ := s.Repo.LoadMeta(sceneID)
	if meta == nil {
		meta = &models.VideoMeta{SceneID: sceneID, CreatedAt: time.Now()}
	}
	meta.CurrentVideoVersion = timeline.VideoVersion
	meta.ComicSnapshotID = timeline.ComicSnapshot.ComicSnapshotID
	meta.SourceVersionHash = timeline.ComicSnapshot.SourceVersionHash
	meta.Provider = timeline.Provider
	meta.Model = timeline.Model
	meta.TimelineStatus = "built"
	meta.GenerationStatus = "running"
	meta.RenderStatus = "running"
	meta.ClipCount = len(timeline.Clips)
	meta.CompletedClipCount = completed
	meta.LastTaskID = taskID
	if result != nil {
		meta.LastProviderTaskID = result.ProviderTaskID
		meta.LastProviderStatus = result.ProviderStatus
	}
	meta.UpdatedAt = time.Now()
	return s.Repo.SaveMeta(sceneID, meta)
}

func (s *VideoService) updateMetaCompleted(sceneID string, taskID string, timeline *models.VideoTimeline, result *models.VideoClipResult, renderArtifactPath string, renderArtifactType string, degraded bool) error {
	meta, _ := s.Repo.LoadMeta(sceneID)
	if meta == nil {
		meta = &models.VideoMeta{SceneID: sceneID, CreatedAt: time.Now()}
	}
	meta.CurrentVideoVersion = timeline.VideoVersion
	meta.ComicSnapshotID = timeline.ComicSnapshot.ComicSnapshotID
	meta.SourceVersionHash = timeline.ComicSnapshot.SourceVersionHash
	meta.Provider = timeline.Provider
	meta.Model = timeline.Model
	meta.TimelineStatus = "built"
	meta.GenerationStatus = map[bool]string{true: "completed_degraded", false: "completed"}[degraded]
	meta.RenderStatus = "completed"
	meta.ClipCount = len(timeline.Clips)
	meta.CompletedClipCount = len(timeline.Clips)
	meta.LastTaskID = taskID
	if result != nil {
		meta.LastProviderTaskID = result.ProviderTaskID
		meta.LastProviderStatus = result.ProviderStatus
	} else if meta.LastProviderStatus == "" {
		meta.LastProviderStatus = "mock_completed"
	}
	meta.RenderArtifactPath = strings.TrimSpace(renderArtifactPath)
	meta.RenderArtifactType = strings.TrimSpace(renderArtifactType)
	meta.Degraded = degraded
	meta.LastError = ""
	meta.UpdatedAt = time.Now()
	return s.Repo.SaveMeta(sceneID, meta)
}

func (s *VideoService) updateMetaRegenerateRunning(sceneID string, taskID string, timeline *models.VideoTimeline) error {
	meta, _ := s.Repo.LoadMeta(sceneID)
	if meta == nil {
		meta = &models.VideoMeta{SceneID: sceneID, CreatedAt: time.Now()}
	}
	meta.CurrentVideoVersion = timeline.VideoVersion
	meta.ComicSnapshotID = timeline.ComicSnapshot.ComicSnapshotID
	meta.SourceVersionHash = timeline.ComicSnapshot.SourceVersionHash
	meta.Provider = timeline.Provider
	meta.Model = timeline.Model
	meta.TimelineStatus = "built"
	meta.GenerationStatus = "running"
	meta.RenderStatus = "running"
	meta.ClipCount = len(timeline.Clips)
	if meta.CompletedClipCount <= 0 {
		meta.CompletedClipCount = len(timeline.Clips)
	}
	meta.LastTaskID = taskID
	meta.LastError = ""
	meta.UpdatedAt = time.Now()
	return s.Repo.SaveMeta(sceneID, meta)
}

func (s *VideoService) updateMetaFailure(sceneID string, taskID string, result *models.VideoClipResult, cause error) error {
	meta, _ := s.Repo.LoadMeta(sceneID)
	if meta == nil {
		meta = &models.VideoMeta{SceneID: sceneID, CreatedAt: time.Now()}
	}
	meta.LastTaskID = taskID
	meta.GenerationStatus = "failed"
	meta.RenderStatus = "failed"
	meta.Degraded = false
	if result != nil {
		meta.LastProviderTaskID = result.ProviderTaskID
		meta.LastProviderStatus = result.ProviderStatus
	}
	if cause != nil {
		meta.LastError = cause.Error()
	}
	meta.UpdatedAt = time.Now()
	return s.Repo.SaveMeta(sceneID, meta)
}

func hashVideoSource(src []videoSourceFrame) (string, error) {
	b, err := json.Marshal(src)
	if err != nil {
		return "", err
	}
	return sha256Hex(b), nil
}

func findTimelineClip(timeline *models.VideoTimeline, frameID string) (models.VideoTimelineClip, error) {
	if timeline == nil {
		return models.VideoTimelineClip{}, ErrVideoTimelineNotFound
	}
	for _, clip := range timeline.Clips {
		if clip.FrameID == frameID {
			return clip, nil
		}
	}
	return models.VideoTimelineClip{}, ErrVideoFrameNotFound
}

func (s *VideoService) generateClip(ctx context.Context, tracker *ProgressTracker, timeline *models.VideoTimeline, clip models.VideoTimelineClip, progress int, message string) (*models.VideoClipResult, error) {
	if s.Provider != nil {
		return s.generateClipWithProvider(ctx, tracker, timeline, clip, progress, message)
	}
	providerTaskID := fmt.Sprintf("mock_%s_%d", clip.FrameID, time.Now().UnixNano())
	if tracker != nil {
		tracker.EmitProgressEventWithMeta(progress, message, VideoJobTypeClipGenerate, clip.FrameID, &ProgressEventMeta{
			Phase:          VideoJobTypeClipGenerate,
			VideoVersion:   timeline.VideoVersion,
			StageIndex:     3,
			StageTotal:     videoStageTotal,
			ProviderTaskID: providerTaskID,
			ProviderStatus: "mock_running",
		})
	}
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}
	return &models.VideoClipResult{
		FrameID:        clip.FrameID,
		VideoVersion:   timeline.VideoVersion,
		AttemptCount:   1,
		ProviderTaskID: providerTaskID,
		Status:         "mock_completed",
		ProviderStatus: "mock_completed",
		ElapsedMS:      1,
		GeneratedAt:    time.Now(),
	}, nil
}

func (s *VideoService) composeFallbackRender(sceneID string, timeline *models.VideoTimeline, clipResults map[string]*models.VideoClipResult) (string, string, error) {
	if timeline == nil {
		return "", "", ErrVideoTimelineNotFound
	}
	if s.Repo == nil {
		return "", "", ErrVideoRepositoryNotReady
	}
	filename := fmt.Sprintf("preview_%s.html", timeline.VideoVersion)
	content := s.buildFallbackRenderHTML(sceneID, timeline, clipResults)
	artifactPath, err := s.Repo.SaveRenderArtifact(sceneID, timeline.VideoVersion, filename, []byte(content))
	if err != nil {
		return "", "", err
	}
	return artifactPath, "html_slideshow", nil
}

func (s *VideoService) composeRenderArtifact(sceneID string, timeline *models.VideoTimeline, clipResults map[string]*models.VideoClipResult) (string, string, error) {
	if s.canComposeMergedMP4Render(timeline, clipResults) {
		artifactPath, artifactType, err := s.composeMergedMP4Render(sceneID, timeline, clipResults)
		if err == nil {
			return artifactPath, artifactType, nil
		}
	}
	if s.canComposeMergedRender(timeline, clipResults) {
		return s.composeMergedRender(sceneID, timeline, clipResults)
	}
	return s.composeFallbackRender(sceneID, timeline, clipResults)
}

func (s *VideoService) canComposeMergedMP4Render(timeline *models.VideoTimeline, clipResults map[string]*models.VideoClipResult) bool {
	if timeline == nil || len(timeline.Clips) == 0 {
		return false
	}
	if _, err := s.resolveFFmpegBinary(); err != nil {
		return false
	}
	for _, clip := range timeline.Clips {
		result := clipResults[clip.FrameID]
		if result == nil || strings.TrimSpace(result.Status) != "completed" {
			return false
		}
		if strings.TrimSpace(result.LocalPath) == "" {
			return false
		}
	}
	return true
}

func (s *VideoService) canComposeMergedRender(timeline *models.VideoTimeline, clipResults map[string]*models.VideoClipResult) bool {
	if timeline == nil || len(timeline.Clips) == 0 {
		return false
	}
	for _, clip := range timeline.Clips {
		result := clipResults[clip.FrameID]
		if result == nil || strings.TrimSpace(result.Status) != "completed" {
			return false
		}
		if strings.TrimSpace(result.LocalPath) == "" && strings.TrimSpace(result.VideoURL) == "" {
			return false
		}
	}
	return true
}

func (s *VideoService) composeMergedRender(sceneID string, timeline *models.VideoTimeline, clipResults map[string]*models.VideoClipResult) (string, string, error) {
	if timeline == nil {
		return "", "", ErrVideoTimelineNotFound
	}
	if s.Repo == nil {
		return "", "", ErrVideoRepositoryNotReady
	}
	filename := fmt.Sprintf("merged_%s.html", timeline.VideoVersion)
	content := s.buildMergedRenderHTML(sceneID, timeline, clipResults)
	artifactPath, err := s.Repo.SaveRenderArtifact(sceneID, timeline.VideoVersion, filename, []byte(content))
	if err != nil {
		return "", "", err
	}
	return artifactPath, "html_stitched_video", nil
}

func (s *VideoService) composeMergedMP4Render(sceneID string, timeline *models.VideoTimeline, clipResults map[string]*models.VideoClipResult) (string, string, error) {
	if timeline == nil {
		return "", "", ErrVideoTimelineNotFound
	}
	if s.Repo == nil {
		return "", "", ErrVideoRepositoryNotReady
	}
	ffmpegBinary, err := s.resolveFFmpegBinary()
	if err != nil {
		return "", "", err
	}
	paths := make([]string, 0, len(timeline.Clips))
	for _, clip := range timeline.Clips {
		result := clipResults[clip.FrameID]
		if result == nil || strings.TrimSpace(result.LocalPath) == "" {
			return "", "", fmt.Errorf("missing local clip asset for %s", clip.FrameID)
		}
		paths = append(paths, filepath.Join(s.Repo.BaseDir, filepath.FromSlash(result.LocalPath)))
	}
	concatFile, cleanup, err := s.createFFmpegConcatList(paths)
	if err != nil {
		return "", "", err
	}
	defer cleanup()
	outputFile, outputCleanup, err := s.createFFmpegOutputFile(timeline.VideoVersion)
	if err != nil {
		return "", "", err
	}
	defer outputCleanup()
	preferAudio := s.shouldAttemptMergedAudio(timeline)
	args := s.buildFFmpegMergedMP4Args(concatFile, outputFile, timeline, preferAudio)
	output, err := s.runFFmpegComposeCommand(ffmpegBinary, args)
	if err != nil {
		if preferAudio {
			fallbackArgs := s.buildFFmpegMergedMP4Args(concatFile, outputFile, timeline, false)
			fallbackOutput, fallbackErr := s.runFFmpegComposeCommand(ffmpegBinary, fallbackArgs)
			if fallbackErr != nil {
				return "", "", fmt.Errorf("ffmpeg compose failed: %w: %s", fallbackErr, strings.TrimSpace(fallbackOutput))
			}
		} else {
			return "", "", fmt.Errorf("ffmpeg compose failed: %w: %s", err, strings.TrimSpace(output))
		}
	}
	content, err := os.ReadFile(outputFile)
	if err != nil {
		return "", "", err
	}
	filename := fmt.Sprintf("preview_%s.mp4", timeline.VideoVersion)
	artifactPath, err := s.Repo.SaveRenderArtifact(sceneID, timeline.VideoVersion, filename, content)
	if err != nil {
		return "", "", err
	}
	return artifactPath, "mp4_stitched_video", nil
}

func (s *VideoService) shouldAttemptMergedAudio(timeline *models.VideoTimeline) bool {
	if timeline == nil {
		return false
	}
	if timeline.AudioEnabled {
		return true
	}
	switch strings.ToLower(strings.TrimSpace(timeline.Model)) {
	case "wan2.6-i2v-flash":
		return true
	default:
		return false
	}
}

func (s *VideoService) buildFFmpegMergedMP4Args(concatFile string, outputFile string, timeline *models.VideoTimeline, includeAudio bool) []string {
	playbackRate := s.resolveMergedPlaybackRate(timeline)
	args := []string{
		"-y",
		"-f", "concat",
		"-safe", "0",
		"-i", concatFile,
		"-map", "0:v:0",
		"-vf", fmt.Sprintf("setpts=PTS/%.6f", playbackRate),
		"-r", fmt.Sprintf("%d", timeline.FPS),
		"-pix_fmt", "yuv420p",
		"-movflags", "+faststart",
		"-c:v", "libx264",
	}
	if includeAudio {
		args = append(args,
			"-map", "0:a:0?",
			"-af", buildFFmpegATempoFilter(playbackRate),
			"-c:a", "aac",
		)
	} else {
		args = append(args, "-an")
	}
	args = append(args, outputFile)
	return args
}

func (s *VideoService) runFFmpegComposeCommand(ffmpegBinary string, args []string) (string, error) {
	cmd := s.commandRunner()(context.Background(), ffmpegBinary, args...)
	output, err := cmd.CombinedOutput()
	return string(output), err
}

func buildFFmpegATempoFilter(playbackRate float64) string {
	if playbackRate <= 0 {
		return "atempo=1.0"
	}
	remaining := playbackRate
	parts := make([]string, 0, 4)
	for remaining > 2.0 {
		parts = append(parts, "atempo=2.0")
		remaining /= 2.0
	}
	for remaining < 0.5 {
		parts = append(parts, "atempo=0.5")
		remaining /= 0.5
	}
	parts = append(parts, fmt.Sprintf("atempo=%.6f", remaining))
	return strings.Join(parts, ",")
}

func (s *VideoService) resolveFFmpegBinary() (string, error) {
	binary := strings.TrimSpace(s.FFmpegPath)
	if binary == "" {
		binary = "ffmpeg"
	}
	resolved, err := exec.LookPath(binary)
	if err != nil {
		return "", err
	}
	return resolved, nil
}

func (s *VideoService) commandRunner() func(ctx context.Context, name string, args ...string) *exec.Cmd {
	if s != nil && s.CommandRunner != nil {
		return s.CommandRunner
	}
	return exec.CommandContext
}

func (s *VideoService) createFFmpegConcatList(paths []string) (string, func(), error) {
	file, err := os.CreateTemp("", "scene-intruder-video-concat-*.txt")
	if err != nil {
		return "", func() {}, err
	}
	for _, clipPath := range paths {
		normalized := filepath.ToSlash(clipPath)
		normalized = strings.ReplaceAll(normalized, "'", "'\\''")
		if _, err := file.WriteString("file '" + normalized + "'\n"); err != nil {
			_ = file.Close()
			_ = os.Remove(file.Name())
			return "", func() {}, err
		}
	}
	if err := file.Close(); err != nil {
		_ = os.Remove(file.Name())
		return "", func() {}, err
	}
	return file.Name(), func() { _ = os.Remove(file.Name()) }, nil
}

func (s *VideoService) createFFmpegOutputFile(videoVersion string) (string, func(), error) {
	tempDir := os.TempDir()
	if strings.TrimSpace(tempDir) == "" {
		tempDir = "."
	}
	file, err := os.CreateTemp(tempDir, "scene-intruder-video-render-*.mp4")
	if err != nil {
		return "", func() {}, err
	}
	path := file.Name()
	if err := file.Close(); err != nil {
		_ = os.Remove(path)
		return "", func() {}, err
	}
	return path, func() { _ = os.Remove(path) }, nil
}

func (s *VideoService) resolveMergedPlaybackRate(timeline *models.VideoTimeline) float64 {
	if timeline == nil {
		return 1
	}
	totalDurationSec := 0.0
	for _, clip := range timeline.Clips {
		if clip.DurationSec > 0 {
			totalDurationSec += clip.DurationSec
		}
	}
	targetDurationSec := s.resolveMergedTargetDurationSec(timeline, totalDurationSec)
	if targetDurationSec <= 0 || totalDurationSec <= 0 || totalDurationSec <= targetDurationSec {
		return 1
	}
	return totalDurationSec / targetDurationSec
}

func (s *VideoService) buildMergedRenderHTML(sceneID string, timeline *models.VideoTimeline, clipResults map[string]*models.VideoClipResult) string {
	type mergedClipItem struct {
		FrameID       string  `json:"frame_id"`
		Order         int     `json:"order"`
		SourceURL     string  `json:"source_url"`
		Prompt        string  `json:"prompt,omitempty"`
		DurationSec   float64 `json:"duration_sec"`
		PlaybackRate  float64 `json:"playback_rate"`
		EffectiveSec  float64 `json:"effective_sec"`
		ProviderState string  `json:"provider_status,omitempty"`
	}
	totalDurationSec := 0.0
	for _, clip := range timeline.Clips {
		if clip.DurationSec > 0 {
			totalDurationSec += clip.DurationSec
		}
	}
	targetDurationSec := s.resolveMergedTargetDurationSec(timeline, totalDurationSec)
	playbackRate := 1.0
	if targetDurationSec > 0 && totalDurationSec > targetDurationSec {
		playbackRate = totalDurationSec / targetDurationSec
	}
	items := make([]mergedClipItem, 0, len(timeline.Clips))
	for _, clip := range timeline.Clips {
		result := clipResults[clip.FrameID]
		sourceURL := ""
		providerState := ""
		if result != nil {
			providerState = strings.TrimSpace(result.ProviderStatus)
			if strings.TrimSpace(result.LocalPath) != "" {
				sourceURL = buildComicVideoClipAssetAPIPath(sceneID, clip.FrameID)
			} else {
				sourceURL = strings.TrimSpace(result.VideoURL)
			}
		}
		effectiveSec := clip.DurationSec
		if playbackRate > 1 {
			effectiveSec = clip.DurationSec / playbackRate
		}
		items = append(items, mergedClipItem{
			FrameID:       clip.FrameID,
			Order:         clip.Order,
			SourceURL:     sourceURL,
			Prompt:        clip.Prompt,
			DurationSec:   clip.DurationSec,
			PlaybackRate:  playbackRate,
			EffectiveSec:  effectiveSec,
			ProviderState: providerState,
		})
	}
	playlistJSON, _ := json.Marshal(items)

	var b strings.Builder
	b.WriteString("<!doctype html><html><head><meta charset=\"utf-8\"><title>Merged Video Playback</title><style>")
	b.WriteString("body{font-family:Arial,sans-serif;background:#020617;color:#e2e8f0;margin:0;padding:24px;} .wrap{max-width:1040px;margin:0 auto;} .panel{background:#0f172a;border:1px solid #1e293b;border-radius:18px;padding:18px;margin-bottom:18px;} .meta{color:#94a3b8;font-size:14px;} .hero{display:grid;gap:16px;} @media(min-width:900px){.hero{grid-template-columns:1.4fr 1fr;align-items:start;}} video{width:100%;background:#000;border-radius:16px;min-height:320px;} ol{padding-left:20px;} li{margin:10px 0;} .badge{display:inline-block;margin-right:8px;padding:4px 10px;border-radius:999px;background:#1e293b;font-size:12px;} .muted{color:#94a3b8;} .prompt{white-space:pre-wrap;color:#cbd5e1;} button{background:#2563eb;color:#fff;border:none;border-radius:999px;padding:10px 16px;cursor:pointer;} button.secondary{background:#334155;} </style></head><body><div class=\"wrap\">")
	b.WriteString("<div class=\"panel\"><h1>Merged Video Playback</h1><p class=\"meta\">scene: ")
	b.WriteString(html.EscapeString(sceneID))
	b.WriteString(" · version: ")
	b.WriteString(html.EscapeString(timeline.VideoVersion))
	b.WriteString("</p><div class=\"meta\"><span class=\"badge\">clips: ")
	b.WriteString(html.EscapeString(fmt.Sprintf("%d", len(items))))
	b.WriteString("</span><span class=\"badge\">original: ")
	b.WriteString(html.EscapeString(fmt.Sprintf("%.2fs", totalDurationSec)))
	b.WriteString("</span><span class=\"badge\">merged target: ")
	b.WriteString(html.EscapeString(fmt.Sprintf("%.2fs", targetDurationSec)))
	b.WriteString("</span><span class=\"badge\">playback rate: ")
	b.WriteString(html.EscapeString(fmt.Sprintf("%.2fx", playbackRate)))
	b.WriteString("</span></div></div>")
	b.WriteString("<div class=\"hero\"><div class=\"panel\"><video id=\"merged-player\" controls playsinline preload=\"metadata\"></video><div style=\"margin-top:12px;display:flex;gap:12px;flex-wrap:wrap;\"><button id=\"play-all\">从头播放</button><button id=\"next-clip\" class=\"secondary\">下一段</button><span id=\"status\" class=\"muted\"></span></div></div><div class=\"panel\"><h2>合并规则</h2><p class=\"meta\">当前不是逐分镜分别交付，而是按顺序串联为一个连续播放入口；当总时长超过模型可接受上限时，使用统一 playbackRate 压缩到目标播放时长。</p><ol>")
	for _, item := range items {
		b.WriteString("<li><span class=\"badge\">")
		b.WriteString(html.EscapeString(item.FrameID))
		b.WriteString("</span><span class=\"muted\">")
		b.WriteString(html.EscapeString(fmt.Sprintf("%.2fs → %.2fs", item.DurationSec, item.EffectiveSec)))
		b.WriteString("</span><div class=\"prompt\">")
		b.WriteString(html.EscapeString(item.Prompt))
		b.WriteString("</div></li>")
	}
	b.WriteString("</ol></div></div>")
	b.WriteString("<script>const playlist=")
	b.Write(playlistJSON)
	b.WriteString(";const globalPlaybackRate=")
	b.WriteString(fmt.Sprintf("%.6f", playbackRate))
	b.WriteString(";const player=document.getElementById('merged-player');const status=document.getElementById('status');let currentIndex=0;let timer=null;function clearTimer(){if(timer){clearTimeout(timer);timer=null;}} function setStatus(){if(!playlist.length){status.textContent='没有可播放片段';return;} const clip=playlist[currentIndex]; status.textContent='当前播放：'+clip.frame_id+' · '+(currentIndex+1)+'/'+playlist.length+' · '+globalPlaybackRate.toFixed(2)+'x';} function armFallbackTimer(){clearTimer(); const clip=playlist[currentIndex]; if(!clip){return;} const effective=Math.max(0.15, Number(clip.effective_sec)||0); timer=setTimeout(()=>playIndex(currentIndex+1), Math.ceil(effective*1000)+120);} function playIndex(index){clearTimer(); if(index>=playlist.length){currentIndex=Math.max(playlist.length-1,0); status.textContent='播放完成'; return;} currentIndex=index; const clip=playlist[currentIndex]; if(!clip||!clip.source_url){status.textContent='片段资源缺失：'+(clip?clip.frame_id:'unknown'); return;} player.src=clip.source_url; player.playbackRate=Math.max(1, Number(clip.playback_rate)||1); setStatus(); player.load(); const playPromise=player.play(); if(playPromise&&playPromise.catch){playPromise.catch(()=>{});} armFallbackTimer();} player.addEventListener('ended',()=>playIndex(currentIndex+1)); player.addEventListener('error',()=>playIndex(currentIndex+1)); player.addEventListener('loadedmetadata',()=>{armFallbackTimer();}); document.getElementById('play-all').addEventListener('click',()=>playIndex(0)); document.getElementById('next-clip').addEventListener('click',()=>playIndex(currentIndex+1)); if(playlist.length){playIndex(0);}else{setStatus();}</script>")
	b.WriteString("</div></body></html>")
	return b.String()
}

func (s *VideoService) resolveMergedTargetDurationSec(timeline *models.VideoTimeline, totalDurationSec float64) float64 {
	targetDurationSec := float64(timeline.TargetDurationSec)
	if capSec := mergedPlaybackCapSec(timeline); capSec > 0 && (targetDurationSec <= 0 || targetDurationSec > capSec) {
		targetDurationSec = capSec
	}
	if targetDurationSec <= 0 {
		targetDurationSec = totalDurationSec
	}
	if targetDurationSec <= 0 {
		targetDurationSec = 1
	}
	return targetDurationSec
}

func mergedPlaybackCapSec(timeline *models.VideoTimeline) float64 {
	if timeline == nil {
		return 0
	}
	switch strings.ToLower(strings.TrimSpace(timeline.Model)) {
	case "wan2.6-i2v-flash":
		return 15
	default:
		return 0
	}
}

func (s *VideoService) loadTimelineClipResults(sceneID string, timeline *models.VideoTimeline) (map[string]*models.VideoClipResult, error) {
	clipResults := make(map[string]*models.VideoClipResult, len(timeline.Clips))
	for _, clip := range timeline.Clips {
		result, err := s.LoadClipResult(sceneID, clip.FrameID)
		if err != nil {
			if isLikelyNotExist(err) {
				continue
			}
			return nil, err
		}
		clipResults[clip.FrameID] = result
	}
	return clipResults, nil
}

func (s *VideoService) buildFallbackRenderHTML(sceneID string, timeline *models.VideoTimeline, clipResults map[string]*models.VideoClipResult) string {
	var b strings.Builder
	b.WriteString("<!doctype html><html><head><meta charset=\"utf-8\"><title>Video Fallback Preview</title><style>")
	b.WriteString("body{font-family:Arial,sans-serif;background:#0b1020;color:#f3f4f6;margin:0;padding:24px;} .deck{max-width:960px;margin:0 auto;} .card{background:#111827;border:1px solid #374151;border-radius:16px;padding:16px;margin:0 0 16px;} .badge{display:inline-block;padding:4px 10px;border-radius:999px;background:#1f2937;margin-right:8px;font-size:12px;} img,video{max-width:100%;border-radius:12px;background:#000;} .meta{color:#9ca3af;font-size:13px;} .prompt{white-space:pre-wrap;background:#0f172a;padding:12px;border-radius:12px;} a{color:#93c5fd;} </style></head><body><div class=\"deck\">")
	b.WriteString("<h1>Video Fallback Preview</h1>")
	b.WriteString("<p class=\"meta\">scene: ")
	b.WriteString(html.EscapeString(sceneID))
	b.WriteString(" · version: ")
	b.WriteString(html.EscapeString(timeline.VideoVersion))
	b.WriteString("</p>")
	for _, clip := range timeline.Clips {
		result := clipResults[clip.FrameID]
		status := "pending"
		videoURL := ""
		attemptCount := 0
		errorMessage := ""
		if result != nil {
			if strings.TrimSpace(result.Status) != "" {
				status = result.Status
			}
			videoURL = strings.TrimSpace(result.VideoURL)
			if strings.TrimSpace(result.LocalPath) != "" {
				videoURL = buildComicVideoClipAssetAPIPath(sceneID, clip.FrameID)
			}
			attemptCount = result.AttemptCount
			errorMessage = strings.TrimSpace(result.ErrorMessage)
		}
		imageSrc := s.resolveFallbackRenderImageSrc(sceneID, clip)
		b.WriteString("<section class=\"card\">")
		b.WriteString("<div><span class=\"badge\">frame: ")
		b.WriteString(html.EscapeString(clip.FrameID))
		b.WriteString("</span><span class=\"badge\">status: ")
		b.WriteString(html.EscapeString(status))
		b.WriteString("</span><span class=\"badge\">duration: ")
		b.WriteString(html.EscapeString(fmt.Sprintf("%.1fs", clip.DurationSec)))
		b.WriteString("</span></div>")
		if videoURL != "" {
			b.WriteString("<p><a href=\"")
			b.WriteString(html.EscapeString(videoURL))
			b.WriteString("\" target=\"_blank\" rel=\"noreferrer\">打开 provider 返回的视频 URL</a></p>")
		}
		if strings.TrimSpace(imageSrc) != "" {
			b.WriteString("<img alt=\"")
			b.WriteString(html.EscapeString(clip.FrameID))
			b.WriteString("\" src=\"")
			b.WriteString(html.EscapeString(imageSrc))
			b.WriteString("\" />")
		}
		b.WriteString("<p class=\"meta\">attempts: ")
		b.WriteString(html.EscapeString(fmt.Sprintf("%d", attemptCount)))
		b.WriteString(" · camera_motion: ")
		b.WriteString(html.EscapeString(clip.CameraMotion))
		b.WriteString(" · transition: ")
		b.WriteString(html.EscapeString(clip.Transition))
		b.WriteString("</p>")
		if errorMessage != "" {
			b.WriteString("<p class=\"meta\">error: ")
			b.WriteString(html.EscapeString(errorMessage))
			b.WriteString("</p>")
		}
		b.WriteString("<div class=\"prompt\">")
		b.WriteString(html.EscapeString(clip.Prompt))
		b.WriteString("</div></section>")
	}
	b.WriteString("</div></body></html>")
	return b.String()
}

func buildComicVideoClipAssetAPIPath(sceneID string, frameID string) string {
	return "/api/scenes/" + url.PathEscape(sceneID) + "/comic/video/clips/" + url.PathEscape(frameID) + "/asset"
}

func (s *VideoService) resolveFallbackRenderImageSrc(sceneID string, clip models.VideoTimelineClip) string {
	if v := strings.TrimSpace(clip.ReferenceImageURL); v != "" {
		return v
	}
	if dataURL, err := s.buildComicFrameDataURL(clip.ImagePath); err == nil && strings.TrimSpace(dataURL) != "" {
		return dataURL
	}
	if strings.TrimSpace(clip.FrameID) == "" {
		return ""
	}
	return "/api/scenes/" + url.PathEscape(sceneID) + "/comic/images/" + url.PathEscape(clip.FrameID)
}

func (s *VideoService) buildComicFrameDataURL(imagePath string) (string, error) {
	rel := filepath.ToSlash(strings.TrimSpace(imagePath))
	if rel == "" || s.ComicRepo == nil {
		return "", nil
	}
	abs := filepath.Join(s.ComicRepo.BaseDir, filepath.FromSlash(rel))
	content, err := os.ReadFile(abs)
	if err != nil {
		return "", err
	}
	contentType := mime.TypeByExtension(strings.ToLower(filepath.Ext(abs)))
	if strings.TrimSpace(contentType) == "" {
		contentType = "image/png"
	}
	return "data:" + contentType + ";base64," + base64.StdEncoding.EncodeToString(content), nil
}

func (s *VideoService) generateClipWithProvider(ctx context.Context, tracker *ProgressTracker, timeline *models.VideoTimeline, clip models.VideoTimelineClip, progress int, message string) (*models.VideoClipResult, error) {
	maxRetries := s.MaxClipRetries
	if maxRetries < 0 {
		maxRetries = 0
	}
	var lastResult *models.VideoClipResult
	var lastErr error
	for attempt := 1; attempt <= maxRetries+1; attempt++ {
		result, err := s.generateClipWithProviderAttempt(ctx, tracker, timeline, clip, progress, message, attempt)
		if err == nil {
			return result, nil
		}
		lastResult = result
		lastErr = err
		if attempt > maxRetries {
			return lastResult, lastErr
		}
		if tracker != nil {
			providerTaskID := ""
			providerStatus := "retrying"
			if lastResult != nil {
				providerTaskID = lastResult.ProviderTaskID
				if strings.TrimSpace(lastResult.ProviderStatus) != "" {
					providerStatus = lastResult.ProviderStatus
				}
			}
			tracker.EmitProgressEventWithMeta(progress, fmt.Sprintf("Retrying clip %s (%d/%d)", clip.FrameID, attempt, maxRetries), VideoJobTypeClipGenerate, clip.FrameID, &ProgressEventMeta{
				Phase:          VideoJobTypeClipGenerate,
				VideoVersion:   timeline.VideoVersion,
				StageIndex:     3,
				StageTotal:     videoStageTotal,
				ProviderTaskID: providerTaskID,
				ProviderStatus: providerStatus,
			})
		}
	}
	return lastResult, lastErr
}

func (s *VideoService) generateClipWithProviderAttempt(ctx context.Context, tracker *ProgressTracker, timeline *models.VideoTimeline, clip models.VideoTimelineClip, progress int, message string, attempt int) (*models.VideoClipResult, error) {
	started := time.Now()
	req, err := s.buildClipRequest(timeline, clip)
	if err != nil {
		return s.buildFailedClipResult(timeline, clip, nil, attempt, started, "request_invalid", err), err
	}
	cacheKey, err := buildVideoClipCacheKey(req)
	if err != nil {
		return s.buildFailedClipResult(timeline, clip, nil, attempt, started, "cache_key_invalid", err), err
	}
	if s.ClipCacheEnabled {
		cached, err := s.tryReuseCachedClip(sceneIDFromTimeline(timeline), timeline.VideoVersion, clip.FrameID, cacheKey)
		if err != nil {
			return s.buildFailedClipResult(timeline, clip, nil, attempt, started, "cache_reuse_failed", err), err
		}
		if cached != nil {
			if tracker != nil {
				tracker.EmitProgressEventWithMeta(progress, message+" (cache hit)", VideoJobTypeClipGenerate, clip.FrameID, &ProgressEventMeta{
					Phase:          VideoJobTypeClipGenerate,
					VideoVersion:   timeline.VideoVersion,
					StageIndex:     3,
					StageTotal:     videoStageTotal,
					ProviderTaskID: cached.ProviderTaskID,
					ProviderStatus: "cache_hit",
				})
			}
			return cached, nil
		}
	}
	task, err := s.Provider.SubmitClipTask(ctx, req)
	if err != nil {
		return s.buildFailedClipResult(timeline, clip, nil, attempt, started, "submit_failed", err), err
	}
	providerStatus := strings.TrimSpace(task.ProviderStatus)
	if providerStatus == "" {
		providerStatus = strings.TrimSpace(task.Status)
	}
	if tracker != nil {
		tracker.EmitProgressEventWithMeta(progress, message, VideoJobTypeClipGenerate, clip.FrameID, &ProgressEventMeta{
			Phase:          VideoJobTypeClipGenerate,
			VideoVersion:   timeline.VideoVersion,
			StageIndex:     3,
			StageTotal:     videoStageTotal,
			ProviderTaskID: strings.TrimSpace(task.TaskID),
			ProviderStatus: providerStatus,
		})
	}
	finalTask, err := s.pollProviderTask(ctx, tracker, timeline.VideoVersion, clip.FrameID, progress, message, task)
	if err != nil {
		return s.buildFailedClipResult(timeline, clip, finalTask, attempt, started, "poll_failed", err), err
	}
	providerStatus = strings.TrimSpace(finalTask.ProviderStatus)
	if providerStatus == "" {
		providerStatus = strings.TrimSpace(finalTask.Status)
	}
	result := &models.VideoClipResult{
		FrameID:         clip.FrameID,
		VideoVersion:    timeline.VideoVersion,
		CacheKey:        cacheKey,
		AttemptCount:    attempt,
		ProviderTaskID:  strings.TrimSpace(finalTask.TaskID),
		ProviderStatus:  providerStatus,
		Status:          normalizeProviderTaskResultStatus(finalTask.Status),
		VideoURL:        strings.TrimSpace(finalTask.ResultURL),
		ElapsedMS:       time.Since(started).Milliseconds(),
		GeneratedAt:     time.Now(),
		ProviderPayload: finalTask.Raw,
	}
	if strings.TrimSpace(finalTask.ResultURL) != "" {
		localPath, err := s.downloadProviderVideo(ctx, timeline.SceneID, timeline.VideoVersion, clip.FrameID, finalTask.ResultURL)
		if err != nil {
			failed := s.buildFailedClipResult(timeline, clip, finalTask, attempt, started, "download_failed", err)
			failed.CacheKey = cacheKey
			failed.ProviderStatus = "download_failed"
			failed.VideoURL = strings.TrimSpace(finalTask.ResultURL)
			return failed, err
		}
		result.LocalPath = localPath
	}
	return result, nil
}

func (s *VideoService) buildFailedClipResult(timeline *models.VideoTimeline, clip models.VideoTimelineClip, task *models.VideoProviderTask, attempt int, started time.Time, fallbackStatus string, cause error) *models.VideoClipResult {
	providerTaskID := ""
	providerStatus := fallbackStatus
	providerPayload := map[string]interface{}(nil)
	if task != nil {
		providerTaskID = strings.TrimSpace(task.TaskID)
		providerPayload = task.Raw
		if v := strings.TrimSpace(task.ProviderStatus); v != "" {
			providerStatus = v
		} else if v := strings.TrimSpace(task.Status); v != "" {
			providerStatus = v
		}
	}
	result := &models.VideoClipResult{
		FrameID:         clip.FrameID,
		VideoVersion:    timeline.VideoVersion,
		CacheKey:        "",
		AttemptCount:    attempt,
		ProviderTaskID:  providerTaskID,
		ProviderStatus:  providerStatus,
		Status:          "failed",
		ElapsedMS:       time.Since(started).Milliseconds(),
		GeneratedAt:     time.Now(),
		ProviderPayload: providerPayload,
	}
	if cause != nil {
		result.ErrorMessage = cause.Error()
	}
	return result
}

func buildVideoClipCacheKey(req models.VideoClipRequest) (string, error) {
	type cachePayload struct {
		SceneID            string  `json:"scene_id"`
		ComicSnapshotID    string  `json:"comic_snapshot_id"`
		FrameID            string  `json:"frame_id"`
		Model              string  `json:"model,omitempty"`
		Prompt             string  `json:"prompt"`
		NegativePrompt     string  `json:"negative_prompt,omitempty"`
		ReferenceImagePath string  `json:"reference_image_path,omitempty"`
		ReferenceImageURL  string  `json:"reference_image_url,omitempty"`
		AudioURL           string  `json:"audio_url,omitempty"`
		DurationSec        float64 `json:"duration_sec,omitempty"`
		Resolution         string  `json:"resolution,omitempty"`
		AudioEnabled       bool    `json:"audio_enabled,omitempty"`
		ShotType           string  `json:"shot_type,omitempty"`
		PromptExtend       bool    `json:"prompt_extend,omitempty"`
		CameraMotion       string  `json:"camera_motion,omitempty"`
		TransitionHint     string  `json:"transition_hint,omitempty"`
		ContinuityMode     string  `json:"continuity_mode,omitempty"`
		PreviousFramePath  string  `json:"previous_frame_path,omitempty"`
		NextFramePath      string  `json:"next_frame_path,omitempty"`
	}
	b, err := json.Marshal(cachePayload{
		SceneID:            req.SceneID,
		ComicSnapshotID:    req.ComicSnapshotID,
		FrameID:            req.FrameID,
		Model:              req.Model,
		Prompt:             req.Prompt,
		NegativePrompt:     req.NegativePrompt,
		ReferenceImagePath: req.ReferenceImagePath,
		ReferenceImageURL:  req.ReferenceImageURL,
		AudioURL:           req.AudioURL,
		DurationSec:        req.DurationSec,
		Resolution:         req.Resolution,
		AudioEnabled:       req.AudioEnabled,
		ShotType:           req.ShotType,
		PromptExtend:       req.PromptExtend,
		CameraMotion:       req.CameraMotion,
		TransitionHint:     req.TransitionHint,
		ContinuityMode:     req.ContinuityMode,
		PreviousFramePath:  req.PreviousFramePath,
		NextFramePath:      req.NextFramePath,
	})
	if err != nil {
		return "", err
	}
	return sha256Hex(b), nil
}

func (s *VideoService) tryReuseCachedClip(sceneID string, videoVersion string, frameID string, cacheKey string) (*models.VideoClipResult, error) {
	if s == nil || s.Repo == nil || strings.TrimSpace(cacheKey) == "" {
		return nil, nil
	}
	prior, err := s.LoadClipResult(sceneID, frameID)
	if err != nil {
		if isLikelyNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	if prior == nil || strings.TrimSpace(prior.Status) != "completed" {
		return nil, nil
	}
	if strings.TrimSpace(prior.CacheKey) == "" || strings.TrimSpace(prior.CacheKey) != strings.TrimSpace(cacheKey) {
		return nil, nil
	}
	if strings.TrimSpace(prior.LocalPath) == "" {
		return nil, nil
	}
	absPath := filepath.Join(s.Repo.BaseDir, filepath.FromSlash(prior.LocalPath))
	content, err := os.ReadFile(absPath)
	if err != nil {
		if isLikelyNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	localPath, err := s.Repo.SaveClipAsset(sceneID, videoVersion, filepath.Base(prior.LocalPath), content)
	if err != nil {
		return nil, err
	}
	return &models.VideoClipResult{
		FrameID:         frameID,
		VideoVersion:    videoVersion,
		CacheKey:        cacheKey,
		AttemptCount:    prior.AttemptCount,
		ProviderTaskID:  prior.ProviderTaskID,
		ProviderStatus:  "cache_hit",
		Status:          "completed",
		VideoURL:        prior.VideoURL,
		LocalPath:       localPath,
		ElapsedMS:       0,
		FromCache:       true,
		GeneratedAt:     time.Now(),
		ProviderPayload: prior.ProviderPayload,
	}, nil
}

func sceneIDFromTimeline(timeline *models.VideoTimeline) string {
	if timeline == nil {
		return ""
	}
	return timeline.SceneID
}

func (s *VideoService) validateTimelineForGeneration(timeline *models.VideoTimeline) error {
	if timeline == nil {
		return ErrVideoTimelineNotFound
	}
	if s.Provider == nil {
		return nil
	}
	for _, clip := range timeline.Clips {
		if _, err := s.buildClipRequest(timeline, clip); err != nil {
			return err
		}
	}
	return nil
}

func (s *VideoService) buildClipRequest(timeline *models.VideoTimeline, clip models.VideoTimelineClip) (models.VideoClipRequest, error) {
	referenceImageURL, err := s.resolveReferenceImageURL(context.Background(), timeline.SceneID, clip.FrameID, timeline.Provider, clip.Model, clip.ReferenceImageURL, nil)
	if err != nil {
		return models.VideoClipRequest{}, err
	}
	if strings.TrimSpace(referenceImageURL) == "" && providerRequiresReferenceImage(timeline.Provider) {
		return models.VideoClipRequest{}, fmt.Errorf("%w: 请显式传入 image_url/img_url，或在 settings/config 中配置 video_config.public_base_url", ErrVideoReferenceImageURLRequired)
	}
	return models.VideoClipRequest{
		SceneID:            timeline.SceneID,
		VideoVersion:       timeline.VideoVersion,
		ComicSnapshotID:    timeline.ComicSnapshot.ComicSnapshotID,
		FrameID:            clip.FrameID,
		Model:              clip.Model,
		Prompt:             clip.Prompt,
		NegativePrompt:     clip.NegativePrompt,
		ReferenceImagePath: clip.ImagePath,
		ReferenceImageURL:  referenceImageURL,
		AudioURL:           timeline.AudioURL,
		DurationSec:        clip.DurationSec,
		Resolution:         timeline.Resolution,
		AudioEnabled:       timeline.AudioEnabled,
		ShotType:           timeline.ShotType,
		PromptExtend:       timeline.PromptExtend,
		CameraMotion:       clip.CameraMotion,
		TransitionHint:     clip.Transition,
		ContinuityMode:     "comic_snapshot",
	}, nil
}

func (s *VideoService) downloadProviderVideo(ctx context.Context, sceneID string, videoVersion string, frameID string, resultURL string) (string, error) {
	if s.Repo == nil {
		return "", ErrVideoRepositoryNotReady
	}
	parsed, err := url.Parse(strings.TrimSpace(resultURL))
	if err != nil {
		return "", fmt.Errorf("parse provider result url: %w", err)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", fmt.Errorf("unsupported provider result url scheme: %s", parsed.Scheme)
	}
	httpClient := &http.Client{Timeout: s.DownloadTimeout}
	if s.DownloadTimeout <= 0 {
		httpClient.Timeout = 60 * time.Second
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, parsed.String(), nil)
	if err != nil {
		return "", err
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("download provider video failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return "", fmt.Errorf("download provider video failed: status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	content, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read provider video body failed: %w", err)
	}
	filename := frameID + inferVideoFileExtension(parsed.Path, resp.Header.Get("Content-Type"))
	localPath, err := s.Repo.SaveClipAsset(sceneID, videoVersion, filename, content)
	if err != nil {
		return "", err
	}
	return localPath, nil
}

func inferVideoFileExtension(pathValue string, contentType string) string {
	if ext := strings.ToLower(strings.TrimSpace(filepath.Ext(pathValue))); ext != "" && len(ext) <= 8 {
		return ext
	}
	mediaType, _, err := mime.ParseMediaType(strings.TrimSpace(contentType))
	if err == nil {
		switch strings.ToLower(mediaType) {
		case "video/mp4":
			return ".mp4"
		case "video/webm":
			return ".webm"
		case "video/quicktime":
			return ".mov"
		case "video/x-msvideo":
			return ".avi"
		}
	}
	return ".mp4"
}

func (s *VideoService) resolveReferenceImageURL(ctx context.Context, sceneID string, frameID string, providerName string, modelName string, override string, imageContent []byte) (string, error) {
	if v := strings.TrimSpace(override); v != "" {
		return v, nil
	}
	baseURL := strings.TrimSpace(s.PublicBaseURL)
	if baseURL == "" && len(imageContent) > 0 {
		switch strings.ToLower(strings.TrimSpace(providerName)) {
		case "dashscope":
			uploadedURL, err := s.uploadReferenceImage(ctx, sceneID, frameID, modelName, imageContent)
			if uploadedURL != "" {
				return uploadedURL, nil
			}
			if dataURL := buildImageDataURL(imageContent); dataURL != "" {
				return dataURL, nil
			}
			if err != nil {
				return "", err
			}
		case "ark":
			if dataURL := buildImageDataURL(imageContent); dataURL != "" {
				return dataURL, nil
			}
		case "kling":
			if encoded := buildImageBase64(imageContent); encoded != "" {
				return encoded, nil
			}
		}
	}
	if baseURL == "" {
		return "", nil
	}
	return buildComicFramePublicURL(baseURL, sceneID, frameID)
}

func providerRequiresReferenceImage(providerName string) bool {
	provider := strings.ToLower(strings.TrimSpace(providerName))
	return provider != "" && provider != "mock"
}

func (s *VideoService) uploadReferenceImage(ctx context.Context, sceneID string, frameID string, modelName string, imageContent []byte) (string, error) {
	if len(imageContent) == 0 {
		return "", nil
	}
	uploader, ok := s.Provider.(VideoReferenceUploader)
	if !ok || uploader == nil {
		return "", nil
	}
	if ctx == nil {
		ctx = context.Background()
	}
	contentType := strings.TrimSpace(http.DetectContentType(imageContent))
	fileName := strings.TrimSpace(frameID)
	if fileName == "" {
		fileName = "reference"
	}
	fileName += inferImageFileExtension(contentType)
	result, err := uploader.UploadReferenceImage(ctx, models.VideoReferenceUploadRequest{
		SceneID:     sceneID,
		FrameID:     frameID,
		Model:       strings.TrimSpace(modelName),
		FileName:    fileName,
		ContentType: contentType,
		Content:     imageContent,
	})
	if err != nil {
		return "", err
	}
	if result == nil {
		return "", nil
	}
	return strings.TrimSpace(result.URL), nil
}

func inferImageFileExtension(contentType string) string {
	mediaType, _, err := mime.ParseMediaType(strings.TrimSpace(contentType))
	if err == nil {
		switch strings.ToLower(mediaType) {
		case "image/png":
			return ".png"
		case "image/jpeg":
			return ".jpg"
		case "image/webp":
			return ".webp"
		case "image/gif":
			return ".gif"
		}
	}
	return ".png"
}

func buildImageDataURL(content []byte) string {
	if len(content) == 0 {
		return ""
	}
	contentType := strings.TrimSpace(http.DetectContentType(content))
	if contentType == "" || !strings.HasPrefix(strings.ToLower(contentType), "image/") {
		contentType = "image/png"
	}
	return "data:" + contentType + ";base64," + base64.StdEncoding.EncodeToString(content)
}

func buildImageBase64(content []byte) string {
	if len(content) == 0 {
		return ""
	}
	return base64.StdEncoding.EncodeToString(content)
}

func buildComicFramePublicURL(baseURL string, sceneID string, frameID string) (string, error) {
	parsed, err := url.Parse(strings.TrimSpace(baseURL))
	if err != nil {
		return "", fmt.Errorf("%w: %v", ErrVideoPublicBaseURLInvalid, err)
	}
	if strings.TrimSpace(parsed.Scheme) == "" || strings.TrimSpace(parsed.Host) == "" {
		return "", fmt.Errorf("%w: %s", ErrVideoPublicBaseURLInvalid, strings.TrimSpace(baseURL))
	}
	pathPrefix := strings.TrimRight(parsed.Path, "/")
	publicPath := "/api/scenes/" + url.PathEscape(sceneID) + "/comic/images/" + url.PathEscape(frameID)
	if pathPrefix != "" {
		if !strings.HasPrefix(pathPrefix, "/") {
			pathPrefix = "/" + pathPrefix
		}
		parsed.Path = pathPrefix + publicPath
	} else {
		parsed.Path = publicPath
	}
	return parsed.String(), nil
}

func (s *VideoService) pollProviderTask(ctx context.Context, tracker *ProgressTracker, videoVersion string, frameID string, progress int, message string, submitted *models.VideoProviderTask) (*models.VideoProviderTask, error) {
	if submitted == nil {
		return nil, errors.New("provider task required")
	}
	task := submitted
	if isProviderTaskTerminal(task.Status) {
		if normalizeProviderTaskResultStatus(task.Status) == "failed" {
			if task.ErrorMessage != "" {
				return task, errors.New(task.ErrorMessage)
			}
			return task, errors.New("video provider task failed")
		}
		return task, nil
	}
	pollInterval := s.PollInterval
	if pollInterval <= 0 {
		pollInterval = 2 * time.Second
	}
	if cfg, ok := s.Provider.(interface{ ProviderPollInterval() time.Duration }); ok {
		if d := cfg.ProviderPollInterval(); d > 0 {
			pollInterval = d
		}
	}
	maxAttempts := s.MaxPollAttempts
	if maxAttempts <= 0 {
		maxAttempts = 90
	}
	if cfg, ok := s.Provider.(interface{ ProviderMaxPollAttempts() int }); ok {
		if n := cfg.ProviderMaxPollAttempts(); n > 0 {
			maxAttempts = n
		}
	}
	for attempt := 0; attempt < maxAttempts; attempt++ {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(pollInterval):
		}
		polled, err := s.Provider.PollTask(ctx, task.TaskID)
		if err != nil {
			return task, err
		}
		task = polled
		providerStatus := strings.TrimSpace(task.ProviderStatus)
		if providerStatus == "" {
			providerStatus = strings.TrimSpace(task.Status)
		}
		if tracker != nil {
			tracker.EmitProgressEventWithMeta(progress, message, VideoJobTypeClipGenerate, frameID, &ProgressEventMeta{
				Phase:          VideoJobTypeClipGenerate,
				VideoVersion:   videoVersion,
				StageIndex:     3,
				StageTotal:     videoStageTotal,
				ProviderTaskID: strings.TrimSpace(task.TaskID),
				ProviderStatus: providerStatus,
			})
		}
		if isProviderTaskTerminal(task.Status) {
			if normalizeProviderTaskResultStatus(task.Status) == "failed" {
				if task.ErrorMessage != "" {
					return task, errors.New(task.ErrorMessage)
				}
				return task, errors.New("video provider task failed")
			}
			return task, nil
		}
	}
	return task, fmt.Errorf("video provider poll timeout: task_id=%s", strings.TrimSpace(task.TaskID))
}

func isProviderTaskTerminal(status string) bool {
	status = strings.ToLower(strings.TrimSpace(status))
	switch status {
	case "completed", "succeeded", "success", "failed", "error", "cancelled", "canceled":
		return true
	default:
		return false
	}
}

func normalizeProviderTaskResultStatus(status string) string {
	status = strings.ToLower(strings.TrimSpace(status))
	switch status {
	case "completed", "succeeded", "success":
		return "completed"
	case "failed", "error", "cancelled", "canceled":
		return "failed"
	default:
		return status
	}
}

func sha256Hex(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func (s *VideoService) LoadClipResult(sceneID string, frameID string) (*models.VideoClipResult, error) {
	if s.Repo == nil {
		return nil, ErrVideoRepositoryNotReady
	}
	if err := validatePathSegment(frameID); err != nil {
		return nil, err
	}
	videoDir, err := s.Repo.videoDir(sceneID)
	if err != nil {
		return nil, err
	}
	var out models.VideoClipResult
	if err := s.Repo.FileStorage.LoadJSONFile(filepath.Join(videoDir, "clips"), frameID+".json", &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (s *VideoService) CurrentTimelineExists(sceneID string) bool {
	if s.Repo == nil {
		return false
	}
	videoDir, err := s.Repo.videoDir(sceneID)
	if err != nil {
		return false
	}
	_, err = os.Stat(filepath.Join(s.Repo.BaseDir, videoDir, "timeline.json"))
	return err == nil
}

func (s *VideoService) ResetVideoWorkspace(sceneID string) error {
	if s.Repo == nil {
		return ErrVideoRepositoryNotReady
	}
	return s.Repo.DeleteVideoArtifacts(sceneID)
}

func (s *VideoService) UpdateTimelineClip(sceneID string, frameID string, patch models.VideoTimelineClipPatch) (*models.VideoTimeline, error) {
	if s.Repo == nil {
		return nil, ErrVideoRepositoryNotReady
	}
	if err := validatePathSegment(strings.TrimSpace(frameID)); err != nil {
		return nil, err
	}
	timeline, err := s.LoadTimeline(sceneID)
	if err != nil {
		return nil, err
	}
	clipIndex := -1
	for i := range timeline.Clips {
		if strings.TrimSpace(timeline.Clips[i].FrameID) == strings.TrimSpace(frameID) {
			clipIndex = i
			break
		}
	}
	if clipIndex < 0 {
		return nil, ErrVideoFrameNotFound
	}
	if patch.Prompt != nil {
		timeline.Clips[clipIndex].PromptBase = strings.TrimSpace(*patch.Prompt)
	}
	if patch.NegativePrompt != nil {
		timeline.Clips[clipIndex].NegativePrompt = strings.TrimSpace(*patch.NegativePrompt)
	}
	if patch.ReferenceImageURL != nil {
		timeline.Clips[clipIndex].ReferenceImageURL = strings.TrimSpace(*patch.ReferenceImageURL)
	}
	timeline.Clips[clipIndex].PromptOptions = applyVideoPromptOptionsPatch(timeline.Clips[clipIndex].PromptOptions, patch.PromptOptions)
	frameDescription := s.lookupFrameDescription(sceneID, frameID)
	promptBase := strings.TrimSpace(timeline.Clips[clipIndex].PromptBase)
	if promptBase == "" {
		promptBase = strings.TrimSpace(timeline.Clips[clipIndex].Prompt)
		timeline.Clips[clipIndex].PromptBase = promptBase
	}
	timeline.Clips[clipIndex].Prompt = buildEnhancedVideoPrompt(promptBase, frameDescription, timeline.Clips[clipIndex].PromptOptions)
	timeline.UpdatedAt = time.Now()

	meta, _ := s.Repo.LoadMeta(sceneID)
	createdAt := time.Now()
	if meta != nil && !meta.CreatedAt.IsZero() {
		createdAt = meta.CreatedAt
	}
	if err := s.Repo.DeleteVideoArtifacts(sceneID); err != nil {
		return nil, err
	}
	if err := s.Repo.SaveTimeline(sceneID, timeline); err != nil {
		return nil, err
	}
	resetMeta := &models.VideoMeta{
		SceneID:             sceneID,
		CurrentVideoVersion: timeline.VideoVersion,
		ComicSnapshotID:     timeline.ComicSnapshot.ComicSnapshotID,
		SourceVersionHash:   timeline.ComicSnapshot.SourceVersionHash,
		Provider:            timeline.Provider,
		Model:               timeline.Model,
		TimelineStatus:      "built",
		GenerationStatus:    "idle",
		RenderStatus:        "idle",
		ClipCount:           len(timeline.Clips),
		CompletedClipCount:  0,
		CreatedAt:           createdAt,
		UpdatedAt:           time.Now(),
	}
	if err := s.Repo.SaveMeta(sceneID, resetMeta); err != nil {
		return nil, err
	}
	return timeline, nil
}

func (s *VideoService) lookupFrameDescription(sceneID string, frameID string) string {
	if s == nil || s.ComicRepo == nil {
		return ""
	}
	analysis, err := s.ComicRepo.LoadAnalysis(sceneID)
	if err != nil || analysis == nil {
		return ""
	}
	for _, frame := range analysis.Frames {
		if strings.TrimSpace(frame.ID) == strings.TrimSpace(frameID) {
			return strings.TrimSpace(frame.Description)
		}
	}
	return ""
}
