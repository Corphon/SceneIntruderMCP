# Performance Improvements and Suggestions

This document outlines identified performance issues and improvement suggestions for the SceneIntruderMCP codebase.

## Table of Contents
1. [Critical Issues](#critical-issues)
2. [Medium Priority Issues](#medium-priority-issues)
3. [Low Priority Issues](#low-priority-issues)
4. [Code Patterns to Improve](#code-patterns-to-improve)

---

## Critical Issues

### 1. Redundant File I/O in Story Service

**Location:** `internal/services/story_service.go`

**Issue:** Multiple methods read the same story data file within locked sections, even though a caching mechanism exists.

**Example:** In `GetStoryData()` (line 614-647), the method reads the file directly using `os.ReadFile()` instead of using the existing `loadStoryDataSafe()` cached method.

**Current Code:**
```go
func (s *StoryService) GetStoryData(sceneID string, preferences *models.UserPreferences) (*models.StoryData, error) {
    var storyData *models.StoryData

    err := s.lockManager.ExecuteWithSceneLock(sceneID, func() error {
        storyPath := filepath.Join(s.BasePath, sceneID, "story.json")
        // Direct file read instead of using cache
        storyDataBytes, err := os.ReadFile(storyPath)
        // ...
    })
    // ...
}
```

**Suggested Fix:** Use the cached `loadStoryDataSafe()` method consistently:
```go
func (s *StoryService) GetStoryData(sceneID string, preferences *models.UserPreferences) (*models.StoryData, error) {
    storyData, err := s.loadStoryDataSafe(sceneID)
    if err != nil {
        // Initialize if not found
        return s.InitializeStoryForScene(sceneID, preferences)
    }
    return storyData, nil
}
```

**Impact:** Reduces unnecessary disk I/O, especially under concurrent access.

---

### 2. String Concatenation in Loops

**Location:** `internal/services/story_service.go` (line 267-276)

**Issue:** String concatenation inside loops using `+=` creates new string allocations for each iteration.

**Current Code:**
```go
if !isEnglish && len(sceneData.Characters) > 0 {
    characterNames := ""
    for _, char := range sceneData.Characters {
        characterNames += char.Name + " "
    }
    isEnglish = isEnglishText(characterNames)
}
```

**Suggested Fix:** Use `strings.Builder`:
```go
if !isEnglish && len(sceneData.Characters) > 0 {
    var sb strings.Builder
    for _, char := range sceneData.Characters {
        sb.WriteString(char.Name)
        sb.WriteByte(' ')
    }
    isEnglish = isEnglishText(sb.String())
}
```

**Impact:** Reduces memory allocations from O(n²) to O(n) for string building.

---

### 3. MetricsCollector Double Locking

**Location:** `internal/utils/metrics.go`

**Issue:** The `IncrementCounter`, `AddCounter`, `SetGauge`, and `RecordHistogram` methods hold a write lock on the map while also acquiring a mutex on the individual metric, creating unnecessary contention.

**Current Code:**
```go
func (m *MetricsCollector) IncrementCounter(name string) {
    m.mu.Lock()
    defer m.mu.Unlock()
    
    counter, exists := m.counters[name]
    if !exists {
        counter = &Counter{name: name}
        m.counters[name] = counter
    }
    
    counter.mu.Lock()
    defer counter.mu.Unlock()
    counter.value++
}
```

**Suggested Fix:** Use `sync.Map` or atomic operations for counters:
```go
func (m *MetricsCollector) IncrementCounter(name string) {
    m.mu.RLock()
    counter, exists := m.counters[name]
    m.mu.RUnlock()
    
    if !exists {
        m.mu.Lock()
        // Double-check after acquiring write lock
        if counter, exists = m.counters[name]; !exists {
            counter = &Counter{name: name}
            m.counters[name] = counter
        }
        m.mu.Unlock()
    }
    
    atomic.AddInt64(&counter.value, 1)
}
```

**Impact:** Significantly reduces lock contention under high request load.

---

## Medium Priority Issues

### 4. Repeated Language Detection

**Location:** Multiple services

**Issue:** `isEnglishText()` is called multiple times with overlapping text in many functions, performing redundant character iteration.

**Example in `story_service.go`:**
```go
isEnglish := isEnglishText(sceneData.Scene.Name + " " + sceneData.Scene.Description)
// Later in same function
isEnglish := isEnglishText(sceneData.Scene.Name + " " + currentNode.Content + " " + selectedChoice.Text)
```

**Suggested Fix:** Cache language detection result per scene or compute once at the beginning:
```go
type SceneLanguageCache struct {
    cache map[string]bool
    mu    sync.RWMutex
}

func (c *SceneLanguageCache) IsEnglish(sceneID string, textProvider func() string) bool {
    c.mu.RLock()
    if result, exists := c.cache[sceneID]; exists {
        c.mu.RUnlock()
        return result
    }
    c.mu.RUnlock()
    
    result := isEnglishText(textProvider())
    c.mu.Lock()
    c.cache[sceneID] = result
    c.mu.Unlock()
    return result
}
```

**Impact:** Reduces CPU cycles for text processing in multi-operation workflows.

---

### 5. Inefficient Slice Operations in formatCharacters

**Location:** `internal/services/story_service.go` (line 2357-2363)

**Issue:** Using `strings.Builder` but calling `WriteString` with fmt.Sprintf creates intermediate string allocations.

**Current Code:**
```go
func formatCharacters(characters []*models.Character) string {
    var result strings.Builder
    for _, char := range characters {
        result.WriteString(fmt.Sprintf("- %s: %s\n", char.Name, char.Personality))
    }
    return result.String()
}
```

**Suggested Fix:** Pre-allocate and use direct writes:
```go
func formatCharacters(characters []*models.Character) string {
    if len(characters) == 0 {
        return ""
    }
    var result strings.Builder
    // Estimate capacity: "- " + name + ": " + personality + "\n" ~ avg 50 chars
    result.Grow(len(characters) * 50)
    for _, char := range characters {
        result.WriteString("- ")
        result.WriteString(char.Name)
        result.WriteString(": ")
        result.WriteString(char.Personality)
        result.WriteByte('\n')
    }
    return result.String()
}
```

**Impact:** Reduces memory allocations per character.

---

### 6. Unnecessary Cache Invalidation After Update

**Location:** `internal/services/context_service.go` (line 199-212)

**Issue:** After updating the cache, the code immediately invalidates it.

**Current Code:**
```go
// Update cache
s.cacheMutex.Lock()
s.sceneCache[sceneID] = &CachedSceneData{
    SceneData: sceneData,
    Timestamp: time.Now(),
}
s.cacheMutex.Unlock()

// Clear cache to force reload when context changes
s.InvalidateSceneCache(sceneID)
```

**Suggested Fix:** Remove the redundant cache update before invalidation:
```go
// Just invalidate - the next read will reload fresh data
s.InvalidateSceneCache(sceneID)
```

**Impact:** Removes unnecessary lock acquisition and memory allocation.

---

### 7. Synchronous Item Saving in Goroutine Without Error Handling

**Location:** `internal/services/story_service.go` (line 1116-1125)

**Issue:** Item saving is done asynchronously but errors are only logged, which could lead to data loss without proper monitoring.

**Current Code:**
```go
go func() {
    if err := s.ItemService.AddItem(sceneID, item); err != nil {
        fmt.Printf("警告: 保存新物品失败: %v\n", err)
    }
}()
```

**Suggested Fix:** Consider using a channel-based error collector or making the save synchronous for critical items:
```go
// Option 1: Synchronous with context
if s.ItemService != nil {
    if err := s.ItemService.AddItem(sceneID, item); err != nil {
        // Log but don't fail the main operation
        logger.Warn("Failed to save item", "sceneID", sceneID, "item", item.Name, "error", err)
    }
}

// Option 2: Use error channel for async operations
type AsyncResult struct {
    Error error
    Type  string
}
errChan := make(chan AsyncResult, 1)
go func() {
    err := s.ItemService.AddItem(sceneID, item)
    errChan <- AsyncResult{Error: err, Type: "item_save"}
}()
```

---

## Low Priority Issues

### 8. Unbounded Lock Map Growth

**Location:** `internal/services/lock_manager.go`

**Issue:** The `sceneLocks` map can grow unbounded, and cleanup only occurs when the count exceeds 200.

**Current Code:**
```go
func (lm *LockManager) cleanupUnusedLocks() {
    // Only cleanup when lock count is excessive
    if len(lm.sceneLocks) > maxLocks {
        // cleanup logic
    }
}
```

**Suggested Fix:** Add periodic cleanup regardless of count, using LRU-style eviction:
```go
func (lm *LockManager) cleanupUnusedLocks() {
    lm.globalLock.Lock()
    defer lm.globalLock.Unlock()

    now := time.Now()
    for sceneID, lockInfo := range lm.sceneLocks {
        if now.Sub(lockInfo.LastUsed) > lm.lockTTL {
            delete(lm.sceneLocks, sceneID)
        }
    }
}
```

---

### 9. Redundant Nil Check in invalidateSceneCache

**Location:** `internal/services/item_service.go` (line 261-271)

**Issue:** Redundant nil check and double deletion.

**Current Code:**
```go
func (s *ItemService) invalidateSceneCache(sceneID string) {
    s.cacheMutex.Lock()
    defer s.cacheMutex.Unlock()

    // nil check
    if s.itemCache != nil {
        delete(s.itemCache, sceneID)
    }

    delete(s.itemCache, sceneID)  // Duplicate!
}
```

**Suggested Fix:**
```go
func (s *ItemService) invalidateSceneCache(sceneID string) {
    s.cacheMutex.Lock()
    defer s.cacheMutex.Unlock()

    if s.itemCache != nil {
        delete(s.itemCache, sceneID)
    }
}
```

---

### 10. Time.Now() Called Multiple Times

**Location:** Various service methods

**Issue:** `time.Now()` is called multiple times within the same function for related operations.

**Example in `story_service.go`:**
```go
newNode := &models.StoryNode{
    CreatedAt:  time.Now(),
    // ...
}
// Later in same function
newNode.Choices = append(newNode.Choices, models.StoryChoice{
    CreatedAt:    time.Now(),
    // ...
})
```

**Suggested Fix:** Capture time once at the beginning:
```go
now := time.Now()
newNode := &models.StoryNode{
    CreatedAt:  now,
    // ...
}
newNode.Choices = append(newNode.Choices, models.StoryChoice{
    CreatedAt:    now,
    // ...
})
```

---

## Code Patterns to Improve

### Pattern 1: Use sync.Pool for Frequent Allocations

For frequently created objects like `strings.Builder`:
```go
var builderPool = sync.Pool{
    New: func() interface{} {
        return new(strings.Builder)
    },
}

func getBuilder() *strings.Builder {
    return builderPool.Get().(*strings.Builder)
}

func putBuilder(b *strings.Builder) {
    b.Reset()
    builderPool.Put(b)
}
```

### Pattern 2: Batch File Operations

Instead of reading multiple JSON files individually, consider batching:
```go
func (s *SceneService) loadAllCharactersOptimized(sceneID string) ([]*models.Character, error) {
    charactersDir := filepath.Join(s.BasePath, sceneID, "characters")
    
    entries, err := os.ReadDir(charactersDir)
    if err != nil {
        return nil, err
    }
    
    characters := make([]*models.Character, 0, len(entries))
    
    // Use a worker pool for concurrent file reads
    type result struct {
        char *models.Character
        err  error
    }
    
    results := make(chan result, len(entries))
    var wg sync.WaitGroup
    
    for _, entry := range entries {
        if filepath.Ext(entry.Name()) != ".json" {
            continue
        }
        wg.Add(1)
        go func(name string) {
            defer wg.Done()
            path := filepath.Join(charactersDir, name)
            data, err := os.ReadFile(path)
            if err != nil {
                results <- result{err: err}
                return
            }
            var char models.Character
            if err := json.Unmarshal(data, &char); err != nil {
                results <- result{err: err}
                return
            }
            results <- result{char: &char}
        }(entry.Name())
    }
    
    go func() {
        wg.Wait()
        close(results)
    }()
    
    for r := range results {
        if r.err != nil {
            continue // Log error but don't fail
        }
        characters = append(characters, r.char)
    }
    
    return characters, nil
}
```

### Pattern 3: Context Propagation

Ensure all LLM calls propagate context properly for timeout and cancellation:
```go
// Good
ctx, cancel := context.WithTimeout(parentCtx, 90*time.Second)
defer cancel()
resp, err := s.LLMService.CreateChatCompletion(ctx, request)

// Instead of
resp, err := s.LLMService.CreateChatCompletion(context.Background(), request)
```

---

## Summary of Priorities

| Priority | Issue | Estimated Impact |
|----------|-------|------------------|
| High | Redundant File I/O | 20-30% disk I/O reduction |
| High | String Concatenation in Loops | Memory allocation reduction |
| High | MetricsCollector Double Locking | Lock contention reduction |
| Medium | Repeated Language Detection | CPU cycle reduction |
| Medium | Slice Pre-allocation | Memory allocation reduction |
| Medium | Unnecessary Cache Invalidation | Lock contention reduction |
| Low | Unbounded Lock Map | Memory leak prevention |
| Low | Duplicate Code Removal | Code maintainability |
| Low | Time.Now() Consolidation | Minor performance gain |

---

## Implementation Recommendations

1. **Start with High Priority Issues**: These provide the most significant performance gains with minimal code changes.

2. **Add Benchmarks**: Before making changes, add benchmarks to measure the actual impact:
   ```go
   func BenchmarkLoadStoryData(b *testing.B) {
       service := NewStoryService(nil)
       for i := 0; i < b.N; i++ {
           service.GetStoryData("test_scene", nil)
       }
   }
   ```

3. **Profile Before and After**: Use Go's pprof to identify actual bottlenecks:
   ```bash
   go test -cpuprofile cpu.prof -memprofile mem.prof -bench .
   go tool pprof cpu.prof
   ```

4. **Incremental Changes**: Make one change at a time and verify it doesn't break existing functionality.

5. **Monitor in Production**: After deployment, monitor metrics to verify performance improvements.
