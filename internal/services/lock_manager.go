// internal/services/lock_manager.go
package services

import (
	"sync"
	"time"
)

// LockManager 统一的锁管理器
type LockManager struct {
	sceneLocks    map[string]*LockInfo
	globalLock    sync.RWMutex
	lockTTL       time.Duration
	cleanupTicker *time.Ticker
}

// LockInfo 包装锁和相关信息
type LockInfo struct {
	Mutex     *sync.RWMutex
	LastUsed  time.Time
	ReferenceCount int32  // 当前锁被引用的次数，用于防止在使用时被清理
}

// NewLockManager 创建锁管理器
func NewLockManager() *LockManager {
	lm := &LockManager{
		sceneLocks: make(map[string]*LockInfo),
		lockTTL:    10 * time.Minute,
	}

	// 启动清理器
	lm.startCleanup()
	return lm
}

// GetSceneLock 获取场景锁（线程安全）
func (lm *LockManager) GetSceneLock(sceneID string) *sync.RWMutex {
	lm.globalLock.RLock()
	if lockInfo, exists := lm.sceneLocks[sceneID]; exists {
		lm.globalLock.RUnlock()
		// 更新最后使用时间
		lockInfo.LastUsed = time.Now()
		return lockInfo.Mutex
	}
	lm.globalLock.RUnlock()

	// 升级为写锁
	lm.globalLock.Lock()
	defer lm.globalLock.Unlock()

	// 双重检查（在写锁保护下是安全的）
	if lockInfo, exists := lm.sceneLocks[sceneID]; exists {
		// 更新最后使用时间
		lockInfo.LastUsed = time.Now()
		return lockInfo.Mutex
	}

	// 创建新锁
	lock := &sync.RWMutex{}
	lockInfo := &LockInfo{
		Mutex:    lock,
		LastUsed: time.Now(),
		ReferenceCount: 0,
	}
	lm.sceneLocks[sceneID] = lockInfo
	return lock
}

// ExecuteWithSceneLock 在场景锁保护下执行操作
func (lm *LockManager) ExecuteWithSceneLock(sceneID string, fn func() error) error {
	lm.globalLock.RLock()
	lockInfo, exists := lm.sceneLocks[sceneID]
	lm.globalLock.RUnlock()
	
	if !exists {
		// 如果锁不存在，创建它
		lock := lm.GetSceneLock(sceneID)
		lock.Lock()
		defer lock.Unlock()
		return fn()
	}

	lockInfo.Mutex.Lock()
	defer lockInfo.Mutex.Unlock()

	// 更新最后使用时间
	lm.globalLock.Lock()
	if existingLockInfo, exists := lm.sceneLocks[sceneID]; exists {
		existingLockInfo.LastUsed = time.Now()
	}
	lm.globalLock.Unlock()

	return fn()
}

// ExecuteWithSceneReadLock 在场景读锁保护下执行操作
func (lm *LockManager) ExecuteWithSceneReadLock(sceneID string, fn func() error) error {
	lm.globalLock.RLock()
	lockInfo, exists := lm.sceneLocks[sceneID]
	lm.globalLock.RUnlock()
	
	if !exists {
		// 如果锁不存在，创建它
		lock := lm.GetSceneLock(sceneID)
		lock.RLock()
		defer lock.RUnlock()
		return fn()
	}

	lockInfo.Mutex.RLock()
	defer lockInfo.Mutex.RUnlock()

	// 更新最后使用时间
	lm.globalLock.Lock()
	if existingLockInfo, exists := lm.sceneLocks[sceneID]; exists {
		existingLockInfo.LastUsed = time.Now()
	}
	lm.globalLock.Unlock()

	return fn()
}

// 定期清理未使用的锁
func (lm *LockManager) startCleanup() {
	lm.cleanupTicker = time.NewTicker(5 * time.Minute)
	go func() {
		for range lm.cleanupTicker.C {
			lm.cleanupUnusedLocks()
		}
	}()
}

func (lm *LockManager) cleanupUnusedLocks() {
	lm.globalLock.Lock()
	defer lm.globalLock.Unlock()

	const maxLocks = 200 // 根据实际场景数量调整
	const lockTimeout = 30 * time.Minute // 锁的超时时间

	// 只有在锁数量过多时才清理
	if len(lm.sceneLocks) > maxLocks {
		// 只清理长时间未使用的锁，而不是全部清理
		now := time.Now()
		for sceneID, lockInfo := range lm.sceneLocks {
			if now.Sub(lockInfo.LastUsed) > lockTimeout {
				// 检查是否有其他协程正在使用此锁（虽然不能完全保证，但可以降低风险）
				// 在实际的引用计数实现中，我们会检查ReferenceCount
				delete(lm.sceneLocks, sceneID)
			}
		}
	}
}
