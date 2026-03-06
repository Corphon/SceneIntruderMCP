// internal/services/vision_provider.go
package services

import "github.com/Corphon/SceneIntruderMCP/internal/vision"

// Aliases to internal/vision types.
//
// This keeps the services layer stable while allowing provider implementations
// to live in internal/vision/providers without import cycles.
type VisionProvider = vision.VisionProvider
type VisionGenerateOptions = vision.VisionGenerateOptions
type VisionImage = vision.VisionImage
