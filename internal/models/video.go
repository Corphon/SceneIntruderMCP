// internal/models/video.go
package models

import "time"

// ComicSnapshotRef binds a video asset to a replayable comic snapshot instead of a mutable comic edit state.
type ComicSnapshotRef struct {
	SceneID           string    `json:"scene_id"`
	SourceComicID     string    `json:"source_comic_id,omitempty"`
	ComicSnapshotID   string    `json:"comic_snapshot_id"`
	SourceVersionHash string    `json:"source_version_hash,omitempty"`
	CreatedAt         time.Time `json:"created_at"`
}

// VideoConfig captures the minimum configuration needed to build timeline and generation requests.
type VideoConfig struct {
	ComicSnapshotID   string             `json:"comic_snapshot_id,omitempty"`
	Provider          string             `json:"provider,omitempty"`
	Model             string             `json:"model,omitempty"`
	TargetDurationSec int                `json:"target_duration_sec,omitempty"`
	FPS               int                `json:"fps,omitempty"`
	Resolution        string             `json:"resolution,omitempty"`
	Duration          int                `json:"duration,omitempty"`
	AudioEnabled      bool               `json:"audio_enabled,omitempty"`
	ShotType          string             `json:"shot_type,omitempty"`
	PromptExtend      bool               `json:"prompt_extend,omitempty"`
	AudioURL          string             `json:"audio_url,omitempty"`
	ImageURL          string             `json:"img_url,omitempty"`
	PromptOptions     VideoPromptOptions `json:"prompt_options,omitempty"`
}

// VideoPromptOptions captures prompt-assembly controls used to enrich image-to-video prompts.
type VideoPromptOptions struct {
	Language            string `json:"language,omitempty"`
	DialogueStyle       string `json:"dialogue_style,omitempty"`
	DialogueDensity     string `json:"dialogue_density,omitempty"`
	DialogueEmotion     string `json:"dialogue_emotion_intensity,omitempty"`
	MotionStrength      string `json:"motion_strength,omitempty"`
	EnvironmentMotion   string `json:"environment_motion,omitempty"`
	EnvironmentLayer    string `json:"environment_layer_preference,omitempty"`
	ExpressionIntensity string `json:"expression_intensity,omitempty"`
	CameraStyle         string `json:"camera_style,omitempty"`
	CameraPacing        string `json:"camera_pacing,omitempty"`
	CameraTransition    string `json:"camera_transition_style,omitempty"`
	PromptSuffix        string `json:"prompt_suffix,omitempty"`
}

// VideoPromptOptionsPatch captures partial prompt-option updates from the UI.
type VideoPromptOptionsPatch struct {
	Language            *string `json:"language,omitempty"`
	DialogueStyle       *string `json:"dialogue_style,omitempty"`
	DialogueDensity     *string `json:"dialogue_density,omitempty"`
	DialogueEmotion     *string `json:"dialogue_emotion_intensity,omitempty"`
	MotionStrength      *string `json:"motion_strength,omitempty"`
	EnvironmentMotion   *string `json:"environment_motion,omitempty"`
	EnvironmentLayer    *string `json:"environment_layer_preference,omitempty"`
	ExpressionIntensity *string `json:"expression_intensity,omitempty"`
	CameraStyle         *string `json:"camera_style,omitempty"`
	CameraPacing        *string `json:"camera_pacing,omitempty"`
	CameraTransition    *string `json:"camera_transition_style,omitempty"`
	PromptSuffix        *string `json:"prompt_suffix,omitempty"`
}

// VideoTimelineClipPatch captures editable timeline fields from the UI.
type VideoTimelineClipPatch struct {
	Prompt            *string                  `json:"prompt,omitempty"`
	NegativePrompt    *string                  `json:"negative_prompt,omitempty"`
	ReferenceImageURL *string                  `json:"reference_image_url,omitempty"`
	PromptOptions     *VideoPromptOptionsPatch `json:"prompt_options,omitempty"`
}

// VideoTimelineClip is the normalized per-frame clip input used by the independent video module.
type VideoTimelineClip struct {
	FrameID            string             `json:"frame_id"`
	Order              int                `json:"order"`
	PromptBase         string             `json:"prompt_base,omitempty"`
	Prompt             string             `json:"prompt"`
	NegativePrompt     string             `json:"negative_prompt,omitempty"`
	Style              string             `json:"style,omitempty"`
	Model              string             `json:"model,omitempty"`
	ImagePath          string             `json:"image_path,omitempty"`
	ImageHash          string             `json:"image_hash,omitempty"`
	ReferenceImageURL  string             `json:"reference_image_url,omitempty"`
	DurationSec        float64            `json:"duration_sec"`
	Transition         string             `json:"transition,omitempty"`
	CameraMotion       string             `json:"camera_motion,omitempty"`
	AnimationIntensity string             `json:"animation_intensity,omitempty"`
	Narration          string             `json:"narration,omitempty"`
	MusicCue           string             `json:"music_cue,omitempty"`
	SFX                string             `json:"sfx,omitempty"`
	PromptOptions      VideoPromptOptions `json:"prompt_options,omitempty"`
}

// VideoTimeline is the persisted timeline draft for the video module.
type VideoTimeline struct {
	SceneID           string              `json:"scene_id"`
	VideoVersion      string              `json:"video_version"`
	ComicSnapshot     ComicSnapshotRef    `json:"comic_snapshot"`
	Provider          string              `json:"provider,omitempty"`
	Model             string              `json:"model,omitempty"`
	TargetDurationSec int                 `json:"target_duration_sec,omitempty"`
	FPS               int                 `json:"fps,omitempty"`
	Resolution        string              `json:"resolution,omitempty"`
	AudioEnabled      bool                `json:"audio_enabled,omitempty"`
	ShotType          string              `json:"shot_type,omitempty"`
	PromptExtend      bool                `json:"prompt_extend,omitempty"`
	AudioURL          string              `json:"audio_url,omitempty"`
	PromptOptions     VideoPromptOptions  `json:"prompt_options,omitempty"`
	Clips             []VideoTimelineClip `json:"clips"`
	GeneratedAt       time.Time           `json:"generated_at"`
	UpdatedAt         time.Time           `json:"updated_at"`
}

// VideoClipRequest is the normalized request sent to a concrete video provider.
type VideoClipRequest struct {
	SceneID            string  `json:"scene_id"`
	VideoVersion       string  `json:"video_version"`
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

// VideoReferenceUploadRequest captures a local frame image that should be uploaded to a provider-accessible store.
type VideoReferenceUploadRequest struct {
	SceneID     string `json:"scene_id,omitempty"`
	FrameID     string `json:"frame_id,omitempty"`
	Model       string `json:"model,omitempty"`
	FileName    string `json:"file_name,omitempty"`
	ContentType string `json:"content_type,omitempty"`
	Content     []byte `json:"-"`
}

// VideoReferenceUploadResult captures the provider-accessible URL returned after uploading a local frame image.
type VideoReferenceUploadResult struct {
	URL         string `json:"url,omitempty"`
	StoragePath string `json:"storage_path,omitempty"`
}

// VideoClipResult is the persisted result of a single clip generation attempt.
type VideoClipResult struct {
	FrameID         string                 `json:"frame_id"`
	VideoVersion    string                 `json:"video_version"`
	CacheKey        string                 `json:"cache_key,omitempty"`
	AttemptCount    int                    `json:"attempt_count,omitempty"`
	ProviderTaskID  string                 `json:"provider_task_id,omitempty"`
	ProviderStatus  string                 `json:"provider_status,omitempty"`
	Status          string                 `json:"status,omitempty"`
	VideoURL        string                 `json:"video_url,omitempty"`
	LocalPath       string                 `json:"local_path,omitempty"`
	ElapsedMS       int64                  `json:"elapsed_ms,omitempty"`
	FromCache       bool                   `json:"from_cache,omitempty"`
	ErrorMessage    string                 `json:"error_message,omitempty"`
	GeneratedAt     time.Time              `json:"generated_at"`
	ProviderPayload map[string]interface{} `json:"provider_payload,omitempty"`
}

// VideoProviderTask is the provider-facing async task shape used by DashScope-like providers.
type VideoProviderTask struct {
	TaskID         string                 `json:"task_id"`
	Status         string                 `json:"status,omitempty"`
	ResultURL      string                 `json:"result_url,omitempty"`
	ErrorMessage   string                 `json:"error_message,omitempty"`
	ProviderStatus string                 `json:"provider_status,omitempty"`
	Raw            map[string]interface{} `json:"raw,omitempty"`
}

// VideoMeta captures the aggregate state of the current video module instance for a scene.
type VideoMeta struct {
	SceneID             string    `json:"scene_id"`
	CurrentVideoVersion string    `json:"current_video_version,omitempty"`
	ComicSnapshotID     string    `json:"comic_snapshot_id,omitempty"`
	SourceVersionHash   string    `json:"source_version_hash,omitempty"`
	Provider            string    `json:"provider,omitempty"`
	Model               string    `json:"model,omitempty"`
	TimelineStatus      string    `json:"timeline_status,omitempty"`
	GenerationStatus    string    `json:"generation_status,omitempty"`
	RenderStatus        string    `json:"render_status,omitempty"`
	ClipCount           int       `json:"clip_count,omitempty"`
	CompletedClipCount  int       `json:"completed_clip_count,omitempty"`
	LastTaskID          string    `json:"last_task_id,omitempty"`
	LastProviderTaskID  string    `json:"last_provider_task_id,omitempty"`
	LastProviderStatus  string    `json:"last_provider_status,omitempty"`
	RenderArtifactPath  string    `json:"render_artifact_path,omitempty"`
	RenderArtifactType  string    `json:"render_artifact_type,omitempty"`
	Degraded            bool      `json:"degraded,omitempty"`
	LastError           string    `json:"last_error,omitempty"`
	CreatedAt           time.Time `json:"created_at"`
	UpdatedAt           time.Time `json:"updated_at"`
}

// VideoClipOverviewItem is the frontend-friendly clip snapshot returned by GET /comic/video.
type VideoClipOverviewItem struct {
	FrameID           string    `json:"frame_id"`
	Order             int       `json:"order"`
	Prompt            string    `json:"prompt,omitempty"`
	DurationSec       float64   `json:"duration_sec,omitempty"`
	ImagePath         string    `json:"image_path,omitempty"`
	ReferenceImageURL string    `json:"reference_image_url,omitempty"`
	ImageURL          string    `json:"image_url,omitempty"`
	Status            string    `json:"status,omitempty"`
	ProviderStatus    string    `json:"provider_status,omitempty"`
	AttemptCount      int       `json:"attempt_count,omitempty"`
	VideoURL          string    `json:"video_url,omitempty"`
	LocalPath         string    `json:"local_path,omitempty"`
	LocalAssetURL     string    `json:"local_asset_url,omitempty"`
	ErrorMessage      string    `json:"error_message,omitempty"`
	FromCache         bool      `json:"from_cache,omitempty"`
	GeneratedAt       time.Time `json:"generated_at,omitempty"`
}

// VideoRecoveryInfo captures whether a video generation can be resumed after reopen/interruption.
type VideoRecoveryInfo struct {
	Status            string   `json:"status,omitempty"`
	CanResume         bool     `json:"can_resume,omitempty"`
	ResumeFrom        string   `json:"resume_from,omitempty"`
	PendingFrameIDs   []string `json:"pending_frame_ids,omitempty"`
	FailedFrameIDs    []string `json:"failed_frame_ids,omitempty"`
	CompletedFrameIDs []string `json:"completed_frame_ids,omitempty"`
	ActiveTaskID      string   `json:"active_task_id,omitempty"`
	ActiveTaskStatus  string   `json:"active_task_status,omitempty"`
	LastProgress      int      `json:"last_progress,omitempty"`
	LastMessage       string   `json:"last_message,omitempty"`
}

// VideoOverview is the aggregated response used by GET /comic/video.
type VideoOverview struct {
	SceneID                   string                  `json:"scene_id"`
	Timeline                  *VideoTimeline          `json:"timeline,omitempty"`
	Meta                      *VideoMeta              `json:"meta,omitempty"`
	Clips                     []VideoClipOverviewItem `json:"clips,omitempty"`
	Recovery                  *VideoRecoveryInfo      `json:"recovery,omitempty"`
	ComicSnapshotID           string                  `json:"comic_snapshot_id,omitempty"`
	VideoVersion              string                  `json:"video_version,omitempty"`
	RenderArtifactURL         string                  `json:"render_artifact_url,omitempty"`
	IsStaleAgainstLatestComic bool                    `json:"is_stale_against_latest_comic"`
}
