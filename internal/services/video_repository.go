// internal/services/video_repository.go
package services

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/Corphon/SceneIntruderMCP/internal/models"
	"github.com/Corphon/SceneIntruderMCP/internal/storage"
)

var ErrInvalidVideoVersion = errors.New("invalid video version")

// VideoRepository manages the independent video module persistence under data/comics/scene_<id>/video/.
type VideoRepository struct {
	BaseDir     string
	FileStorage *storage.FileStorage
}

func NewVideoRepository(baseDir string) (*VideoRepository, error) {
	fs, err := storage.NewFileStorage(baseDir)
	if err != nil {
		return nil, err
	}
	return &VideoRepository{BaseDir: baseDir, FileStorage: fs}, nil
}

func (r *VideoRepository) sceneDir(sceneID string) (string, error) {
	if err := validatePathSegment(sceneID); err != nil {
		return "", err
	}
	return fmt.Sprintf("scene_%s", sceneID), nil
}

func (r *VideoRepository) videoDir(sceneID string) (string, error) {
	sceneDir, err := r.sceneDir(sceneID)
	if err != nil {
		return "", err
	}
	return filepath.Join(sceneDir, "video"), nil
}

func (r *VideoRepository) versionDir(sceneID string, videoVersion string) (string, error) {
	if err := validatePathSegment(strings.TrimSpace(videoVersion)); err != nil {
		return "", ErrInvalidVideoVersion
	}
	videoDir, err := r.videoDir(sceneID)
	if err != nil {
		return "", err
	}
	return filepath.Join(videoDir, "versions", videoVersion), nil
}

func (r *VideoRepository) EnsureVideoLayout(sceneID string) error {
	videoDir, err := r.videoDir(sceneID)
	if err != nil {
		return err
	}
	for _, dir := range []string{
		filepath.Join(r.BaseDir, videoDir),
		filepath.Join(r.BaseDir, videoDir, "versions"),
		filepath.Join(r.BaseDir, videoDir, "clips"),
		filepath.Join(r.BaseDir, videoDir, "renders"),
		filepath.Join(r.BaseDir, videoDir, "audio"),
		filepath.Join(r.BaseDir, videoDir, "exports"),
	} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return err
		}
	}
	return nil
}

// DeleteVideoArtifacts removes all persisted video artifacts for a scene.
// If the directory does not exist, it returns nil.
func (r *VideoRepository) DeleteVideoArtifacts(sceneID string) error {
	videoDir, err := r.videoDir(sceneID)
	if err != nil {
		return err
	}
	if r.FileStorage != nil {
		if err := r.FileStorage.DeleteDir(videoDir); err != nil {
			if isLikelyNotExist(err) {
				return nil
			}
			return err
		}
		return nil
	}
	abs := filepath.Join(r.BaseDir, videoDir)
	if _, err := os.Stat(abs); err != nil {
		if isLikelyNotExist(err) {
			return nil
		}
		return err
	}
	return os.RemoveAll(abs)
}

func (r *VideoRepository) SaveTimeline(sceneID string, timeline *models.VideoTimeline) error {
	if timeline == nil {
		return errors.New("timeline required")
	}
	if err := r.EnsureVideoLayout(sceneID); err != nil {
		return err
	}
	videoDir, err := r.videoDir(sceneID)
	if err != nil {
		return err
	}
	if err := r.FileStorage.SaveJSONFile(videoDir, "timeline.json", timeline); err != nil {
		return err
	}
	if strings.TrimSpace(timeline.VideoVersion) != "" {
		versionDir, err := r.versionDir(sceneID, timeline.VideoVersion)
		if err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Join(r.BaseDir, versionDir), 0755); err != nil {
			return err
		}
		if err := r.FileStorage.SaveJSONFile(versionDir, "timeline.json", timeline); err != nil {
			return err
		}
	}
	return nil
}

func (r *VideoRepository) LoadTimeline(sceneID string) (*models.VideoTimeline, error) {
	videoDir, err := r.videoDir(sceneID)
	if err != nil {
		return nil, err
	}
	var out models.VideoTimeline
	if err := r.FileStorage.LoadJSONFile(videoDir, "timeline.json", &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (r *VideoRepository) SaveMeta(sceneID string, meta *models.VideoMeta) error {
	if meta == nil {
		return errors.New("meta required")
	}
	if err := r.EnsureVideoLayout(sceneID); err != nil {
		return err
	}
	videoDir, err := r.videoDir(sceneID)
	if err != nil {
		return err
	}
	if err := r.FileStorage.SaveJSONFile(videoDir, "meta.json", meta); err != nil {
		return err
	}
	if strings.TrimSpace(meta.CurrentVideoVersion) != "" {
		versionDir, err := r.versionDir(sceneID, meta.CurrentVideoVersion)
		if err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Join(r.BaseDir, versionDir), 0755); err != nil {
			return err
		}
		if err := r.FileStorage.SaveJSONFile(versionDir, "meta.json", meta); err != nil {
			return err
		}
	}
	return nil
}

func (r *VideoRepository) LoadMeta(sceneID string) (*models.VideoMeta, error) {
	videoDir, err := r.videoDir(sceneID)
	if err != nil {
		return nil, err
	}
	var out models.VideoMeta
	if err := r.FileStorage.LoadJSONFile(videoDir, "meta.json", &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (r *VideoRepository) SaveClipResult(sceneID string, videoVersion string, frameID string, clip *models.VideoClipResult) error {
	if clip == nil {
		return errors.New("clip result required")
	}
	if err := validatePathSegment(frameID); err != nil {
		return fmt.Errorf("invalid frame id: %w", err)
	}
	if err := r.EnsureVideoLayout(sceneID); err != nil {
		return err
	}
	videoDir, err := r.videoDir(sceneID)
	if err != nil {
		return err
	}
	if err := r.FileStorage.SaveJSONFile(filepath.Join(videoDir, "clips"), frameID+".json", clip); err != nil {
		return err
	}
	if strings.TrimSpace(videoVersion) != "" {
		versionDir, err := r.versionDir(sceneID, videoVersion)
		if err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Join(r.BaseDir, versionDir, "clips"), 0755); err != nil {
			return err
		}
		if err := r.FileStorage.SaveJSONFile(filepath.Join(versionDir, "clips"), frameID+".json", clip); err != nil {
			return err
		}
	}
	return nil
}

func (r *VideoRepository) SaveClipAsset(sceneID string, videoVersion string, filename string, content []byte) (string, error) {
	if err := validatePathSegment(strings.TrimSpace(filename)); err != nil {
		return "", fmt.Errorf("invalid clip filename: %w", err)
	}
	if err := r.EnsureVideoLayout(sceneID); err != nil {
		return "", err
	}
	videoDir, err := r.videoDir(sceneID)
	if err != nil {
		return "", err
	}
	if err := r.FileStorage.SaveTextFile(filepath.Join(videoDir, "clips"), filename, content); err != nil {
		return "", err
	}
	if strings.TrimSpace(videoVersion) != "" {
		versionDir, err := r.versionDir(sceneID, videoVersion)
		if err != nil {
			return "", err
		}
		if err := os.MkdirAll(filepath.Join(r.BaseDir, versionDir, "clips"), 0755); err != nil {
			return "", err
		}
		if err := r.FileStorage.SaveTextFile(filepath.Join(versionDir, "clips"), filename, content); err != nil {
			return "", err
		}
	}
	return filepath.ToSlash(filepath.Join("scene_"+sceneID, "video", "clips", filename)), nil
}

func (r *VideoRepository) SaveRenderArtifact(sceneID string, videoVersion string, filename string, content []byte) (string, error) {
	if err := validatePathSegment(strings.TrimSpace(filename)); err != nil {
		return "", fmt.Errorf("invalid render filename: %w", err)
	}
	if err := r.EnsureVideoLayout(sceneID); err != nil {
		return "", err
	}
	videoDir, err := r.videoDir(sceneID)
	if err != nil {
		return "", err
	}
	if err := r.FileStorage.SaveTextFile(filepath.Join(videoDir, "renders"), filename, content); err != nil {
		return "", err
	}
	if strings.TrimSpace(videoVersion) != "" {
		versionDir, err := r.versionDir(sceneID, videoVersion)
		if err != nil {
			return "", err
		}
		if err := os.MkdirAll(filepath.Join(r.BaseDir, versionDir, "renders"), 0755); err != nil {
			return "", err
		}
		if err := r.FileStorage.SaveTextFile(filepath.Join(versionDir, "renders"), filename, content); err != nil {
			return "", err
		}
	}
	return filepath.ToSlash(filepath.Join("scene_"+sceneID, "video", "renders", filename)), nil
}
