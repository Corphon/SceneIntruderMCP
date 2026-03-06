// internal/services/comic_repository.go
package services

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Corphon/SceneIntruderMCP/internal/models"
	"github.com/Corphon/SceneIntruderMCP/internal/storage"
)

var ErrInvalidSceneID = errors.New("invalid scene id")

// ComicRepository 管理 v2 comics 的落盘结构（data/comics/...）。
// Phase1 只实现目录初始化与最小读写能力，后续再扩展版本/frames 等。
type ComicRepository struct {
	BaseDir     string
	FileStorage *storage.FileStorage
}

func NewComicRepository(baseDir string) (*ComicRepository, error) {
	fs, err := storage.NewFileStorage(baseDir)
	if err != nil {
		return nil, err
	}
	return &ComicRepository{BaseDir: baseDir, FileStorage: fs}, nil
}

func (r *ComicRepository) sceneDir(sceneID string) (string, error) {
	if err := validatePathSegment(sceneID); err != nil {
		return "", err
	}
	return fmt.Sprintf("scene_%s", sceneID), nil
}

// DeleteSceneArtifacts removes all persisted comic artifacts for a scene.
// It deletes data/comics/scene_<id>/ recursively.
// If the directory does not exist, it returns nil.
func (r *ComicRepository) DeleteSceneArtifacts(sceneID string) error {
	sceneDir, err := r.sceneDir(sceneID)
	if err != nil {
		return err
	}
	if r.FileStorage != nil {
		if err := r.FileStorage.DeleteDir(sceneDir); err != nil {
			if isLikelyNotExist(err) {
				return nil
			}
			return err
		}
		return nil
	}
	abs := filepath.Join(r.BaseDir, sceneDir)
	if _, err := os.Stat(abs); err != nil {
		if isLikelyNotExist(err) {
			return nil
		}
		return err
	}
	if err := os.RemoveAll(abs); err != nil {
		return err
	}
	return nil
}

func validatePathSegment(segment string) error {
	if strings.TrimSpace(segment) == "" {
		return ErrInvalidSceneID
	}
	if strings.Contains(segment, "..") {
		return ErrInvalidSceneID
	}
	if strings.ContainsAny(segment, `/\\`) {
		return ErrInvalidSceneID
	}
	return nil
}

// EnsureSceneLayout 确保 comics/scene_<id>/ 下的基础目录存在：prompts/images/references。
func (r *ComicRepository) EnsureSceneLayout(sceneID string) (string, error) {
	sceneDir, err := r.sceneDir(sceneID)
	if err != nil {
		return "", err
	}

	// FileStorage.BaseDir 已保证存在；这里创建 scene 级目录结构。
	absSceneDir := filepath.Join(r.BaseDir, sceneDir)
	for _, p := range []string{
		absSceneDir,
		filepath.Join(absSceneDir, "prompts"),
		filepath.Join(absSceneDir, "images"),
		filepath.Join(absSceneDir, "references"),
	} {
		if err := os.MkdirAll(p, 0755); err != nil {
			return "", fmt.Errorf("create comics dir failed: %w", err)
		}
	}

	return sceneDir, nil
}

func (r *ComicRepository) SaveMeta(sceneID string, meta interface{}) error {
	sceneDir, err := r.sceneDir(sceneID)
	if err != nil {
		return err
	}
	if _, err := r.EnsureSceneLayout(sceneID); err != nil {
		return err
	}
	return r.FileStorage.SaveJSONFile(sceneDir, "meta.json", meta)
}

func (r *ComicRepository) SaveAnalysis(sceneID string, breakdown *models.ComicBreakdown) error {
	if breakdown == nil {
		return errors.New("analysis required")
	}
	sceneDir, err := r.sceneDir(sceneID)
	if err != nil {
		return err
	}
	if _, err := r.EnsureSceneLayout(sceneID); err != nil {
		return err
	}
	return r.FileStorage.SaveJSONFile(sceneDir, "analysis.json", breakdown)
}

func (r *ComicRepository) LoadAnalysis(sceneID string) (*models.ComicBreakdown, error) {
	sceneDir, err := r.sceneDir(sceneID)
	if err != nil {
		return nil, err
	}
	var out models.ComicBreakdown
	if err := r.FileStorage.LoadJSONFile(sceneDir, "analysis.json", &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (r *ComicRepository) SaveKeyElements(sceneID string, elements *models.ComicKeyElements) error {
	if elements == nil {
		return errors.New("key elements required")
	}
	sceneDir, err := r.sceneDir(sceneID)
	if err != nil {
		return err
	}
	if _, err := r.EnsureSceneLayout(sceneID); err != nil {
		return err
	}
	return r.FileStorage.SaveJSONFile(sceneDir, "key_elements.json", elements)
}

func (r *ComicRepository) LoadKeyElements(sceneID string) (*models.ComicKeyElements, error) {
	sceneDir, err := r.sceneDir(sceneID)
	if err != nil {
		return nil, err
	}
	var out models.ComicKeyElements
	if err := r.FileStorage.LoadJSONFile(sceneDir, "key_elements.json", &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (r *ComicRepository) SavePrompt(sceneID string, frameID string, prompt interface{}) error {
	if err := validatePathSegment(frameID); err != nil {
		return fmt.Errorf("invalid frame id: %w", err)
	}
	sceneDir, err := r.sceneDir(sceneID)
	if err != nil {
		return err
	}
	if _, err := r.EnsureSceneLayout(sceneID); err != nil {
		return err
	}
	return r.FileStorage.SaveJSONFile(filepath.Join(sceneDir, "prompts"), frameID+".json", prompt)
}

func (r *ComicRepository) LoadPrompt(sceneID string, frameID string) (*models.ComicFramePrompt, error) {
	if err := validatePathSegment(frameID); err != nil {
		return nil, fmt.Errorf("invalid frame id: %w", err)
	}
	sceneDir, err := r.sceneDir(sceneID)
	if err != nil {
		return nil, err
	}
	var out models.ComicFramePrompt
	if err := r.FileStorage.LoadJSONFile(filepath.Join(sceneDir, "prompts"), frameID+".json", &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (r *ComicRepository) SaveReference(sceneID string, filename string, content []byte) error {
	if err := validatePathSegment(filename); err != nil {
		return fmt.Errorf("invalid reference filename: %w", err)
	}
	sceneDir, err := r.sceneDir(sceneID)
	if err != nil {
		return err
	}
	if _, err := r.EnsureSceneLayout(sceneID); err != nil {
		return err
	}
	return r.FileStorage.SaveTextFile(filepath.Join(sceneDir, "references"), filename, content)
}

func (r *ComicRepository) LoadReference(sceneID string, filename string) ([]byte, error) {
	if err := validatePathSegment(filename); err != nil {
		return nil, fmt.Errorf("invalid reference filename: %w", err)
	}
	sceneDir, err := r.sceneDir(sceneID)
	if err != nil {
		return nil, err
	}
	return r.FileStorage.LoadTextFile(filepath.Join(sceneDir, "references"), filename)
}

func isLikelyNotExist(err error) bool {
	if err == nil {
		return false
	}
	if os.IsNotExist(err) {
		return true
	}
	msg := strings.ToLower(err.Error())
	if strings.Contains(msg, "文件不存在") {
		return true
	}
	if strings.Contains(msg, "目录不存在") {
		return true
	}
	if strings.Contains(msg, "no such file or directory") {
		return true
	}
	if strings.Contains(msg, "the system cannot find the path specified") {
		return true
	}
	if strings.Contains(msg, "the system cannot find the file specified") {
		return true
	}
	if strings.Contains(msg, "系统找不到指定的路径") {
		return true
	}
	if strings.Contains(msg, "系统找不到指定的文件") {
		return true
	}
	return false
}

// LoadReferenceIndex loads references/index.json. If it does not exist, it returns (nil, err).
func (r *ComicRepository) LoadReferenceIndex(sceneID string) (*models.ComicReferenceIndex, error) {
	sceneDir, err := r.sceneDir(sceneID)
	if err != nil {
		return nil, err
	}
	var out models.ComicReferenceIndex
	if err := r.FileStorage.LoadJSONFile(filepath.Join(sceneDir, "references"), "index.json", &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (r *ComicRepository) SaveReferenceIndex(sceneID string, idx *models.ComicReferenceIndex) error {
	if idx == nil {
		return errors.New("reference index required")
	}
	sceneDir, err := r.sceneDir(sceneID)
	if err != nil {
		return err
	}
	if _, err := r.EnsureSceneLayout(sceneID); err != nil {
		return err
	}
	return r.FileStorage.SaveJSONFile(filepath.Join(sceneDir, "references"), "index.json", idx)
}

// UpsertReferenceIndex updates references/index.json for an element.
func (r *ComicRepository) UpsertReferenceIndex(sceneID, elementID, fileName, contentType string, sizeBytes int64) (*models.ComicReferenceIndex, error) {
	if err := validatePathSegment(elementID); err != nil {
		return nil, fmt.Errorf("invalid element id: %w", err)
	}
	if err := validatePathSegment(fileName); err != nil {
		return nil, fmt.Errorf("invalid reference filename: %w", err)
	}

	idx, err := r.LoadReferenceIndex(sceneID)
	if err != nil {
		if !isLikelyNotExist(err) {
			return nil, err
		}
		idx = &models.ComicReferenceIndex{SceneID: sceneID, References: make(map[string]models.ComicReferenceMeta)}
	}
	if idx.References == nil {
		idx.References = make(map[string]models.ComicReferenceMeta)
	}

	now := time.Now()
	idx.SceneID = sceneID
	idx.References[elementID] = models.ComicReferenceMeta{
		ElementID:   elementID,
		FileName:    fileName,
		ContentType: contentType,
		SizeBytes:   sizeBytes,
		UploadedAt:  now,
	}
	idx.UpdatedAt = now

	if err := r.SaveReferenceIndex(sceneID, idx); err != nil {
		return nil, err
	}
	return idx, nil
}

// SaveReferenceForElement stores a reference image for a given element ID using a standardized filename.
// It returns the saved fileName (e.g. "char_foo.png").
func (r *ComicRepository) SaveReferenceForElement(sceneID, elementID, ext string, content []byte) (fileName string, err error) {
	if err := validatePathSegment(elementID); err != nil {
		return "", fmt.Errorf("invalid element id: %w", err)
	}
	ext = strings.ToLower(strings.TrimSpace(ext))
	if ext == "" || !strings.HasPrefix(ext, ".") {
		return "", errors.New("invalid extension")
	}
	fileName = elementID + ext
	if err := r.SaveReference(sceneID, fileName, content); err != nil {
		return "", err
	}
	return fileName, nil
}

// DeleteReferenceForElement deletes an uploaded reference image and removes it from references/index.json.
// If the index entry exists but the file is missing, it will still remove the index entry.
func (r *ComicRepository) DeleteReferenceForElement(sceneID, elementID string) (*models.ComicReferenceIndex, error) {
	if err := validatePathSegment(elementID); err != nil {
		return nil, fmt.Errorf("invalid element id: %w", err)
	}

	idx, err := r.LoadReferenceIndex(sceneID)
	if err != nil {
		return nil, err
	}
	if idx == nil || idx.References == nil {
		return nil, fmt.Errorf("%w: reference not found", os.ErrNotExist)
	}
	meta, ok := idx.References[elementID]
	if !ok || strings.TrimSpace(meta.FileName) == "" {
		return nil, fmt.Errorf("%w: reference not found", os.ErrNotExist)
	}

	fileName := strings.TrimSpace(meta.FileName)
	if fileName != "" {
		// Best-effort delete: allow cleaning up stale index entries.
		sceneDir, err := r.sceneDir(sceneID)
		if err != nil {
			return nil, err
		}
		if err := r.FileStorage.DeleteFile(filepath.Join(sceneDir, "references"), fileName); err != nil {
			if !isLikelyNotExist(err) {
				return nil, err
			}
		}
	}

	delete(idx.References, elementID)
	idx.SceneID = sceneID
	idx.UpdatedAt = time.Now()

	if err := r.SaveReferenceIndex(sceneID, idx); err != nil {
		return nil, err
	}
	return idx, nil
}

// SaveFrameImage saves raw PNG bytes to comics/scene_<id>/images/<frameID>.png.
func (r *ComicRepository) SaveFrameImage(sceneID string, frameID string, pngBytes []byte) (relativePath string, err error) {
	if err := validatePathSegment(frameID); err != nil {
		return "", fmt.Errorf("invalid frame id: %w", err)
	}
	sceneDir, err := r.sceneDir(sceneID)
	if err != nil {
		return "", err
	}
	if _, err := r.EnsureSceneLayout(sceneID); err != nil {
		return "", err
	}
	filename := frameID + ".png"
	if err := r.FileStorage.SaveTextFile(filepath.Join(sceneDir, "images"), filename, pngBytes); err != nil {
		return "", err
	}
	return filepath.ToSlash(filepath.Join(sceneDir, "images", filename)), nil
}

// LoadFrameImage loads raw PNG bytes from comics/scene_<id>/images/<frameID>.png.
func (r *ComicRepository) LoadFrameImage(sceneID string, frameID string) ([]byte, error) {
	if err := validatePathSegment(frameID); err != nil {
		return nil, fmt.Errorf("invalid frame id: %w", err)
	}
	sceneDir, err := r.sceneDir(sceneID)
	if err != nil {
		return nil, err
	}
	filename := frameID + ".png"
	return r.FileStorage.LoadTextFile(filepath.Join(sceneDir, "images"), filename)
}

// SaveFrameImageSignature saves image render signature metadata to comics/scene_<id>/images/<frameID>.meta.json.
func (r *ComicRepository) SaveFrameImageSignature(sceneID string, frameID string, sig *models.ComicFrameImageSignature) error {
	if sig == nil {
		return errors.New("frame image signature required")
	}
	if err := validatePathSegment(frameID); err != nil {
		return fmt.Errorf("invalid frame id: %w", err)
	}
	sceneDir, err := r.sceneDir(sceneID)
	if err != nil {
		return err
	}
	if _, err := r.EnsureSceneLayout(sceneID); err != nil {
		return err
	}
	return r.FileStorage.SaveJSONFile(filepath.Join(sceneDir, "images"), frameID+".meta.json", sig)
}

// LoadFrameImageSignature loads image render signature metadata from comics/scene_<id>/images/<frameID>.meta.json.
func (r *ComicRepository) LoadFrameImageSignature(sceneID string, frameID string) (*models.ComicFrameImageSignature, error) {
	if err := validatePathSegment(frameID); err != nil {
		return nil, fmt.Errorf("invalid frame id: %w", err)
	}
	sceneDir, err := r.sceneDir(sceneID)
	if err != nil {
		return nil, err
	}
	var out models.ComicFrameImageSignature
	if err := r.FileStorage.LoadJSONFile(filepath.Join(sceneDir, "images"), frameID+".meta.json", &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (r *ComicRepository) SaveMetrics(sceneID string, metrics *models.ComicMetrics) error {
	if metrics == nil {
		return errors.New("metrics required")
	}
	sceneDir, err := r.sceneDir(sceneID)
	if err != nil {
		return err
	}
	if _, err := r.EnsureSceneLayout(sceneID); err != nil {
		return err
	}
	return r.FileStorage.SaveJSONFile(sceneDir, "metrics.json", metrics)
}

func (r *ComicRepository) LoadMetrics(sceneID string) (*models.ComicMetrics, error) {
	sceneDir, err := r.sceneDir(sceneID)
	if err != nil {
		return nil, err
	}
	var out models.ComicMetrics
	if err := r.FileStorage.LoadJSONFile(sceneDir, "metrics.json", &out); err != nil {
		return nil, err
	}
	return &out, nil
}
