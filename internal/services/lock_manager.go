// internal/services/lock_manager.go
package services

import (
	"sync"
	"time"
)

// LockManager 统一的锁管理器
type LockManager struct {
	sceneLocks    map[string]*sync.RWMutex
	globalLock    sync.RWMutex
	lockTTL       time.Duration
	cleanupTicker *time.Ticker
}

// NewLockManager 创建锁管理器
func NewLockManager() *LockManager {
	lm := &LockManager{
		sceneLocks: make(map[string]*sync.RWMutex),
		lockTTL:    10 * time.Minute,
	}

	// 启动清理器
	lm.startCleanup()
	return lm
}

// GetSceneLock 获取场景锁（线程安全）
func (lm *LockManager) GetSceneLock(sceneID string) *sync.RWMutex {
	lm.globalLock.RLock()
	if lock, exists := lm.sceneLocks[sceneID]; exists {
		lm.globalLock.RUnlock()
		return lock
	}
	lm.globalLock.RUnlock()

	// 升级为写锁
	lm.globalLock.Lock()
	defer lm.globalLock.Unlock()

	// 双重检查（在写锁保护下是安全的）
	if lock, exists := lm.sceneLocks[sceneID]; exists {
		return lock
	}

	// 创建新锁
	lock := &sync.RWMutex{}
	lm.sceneLocks[sceneID] = lock
	return lock
}

// ExecuteWithSceneLock 在场景锁保护下执行操作
func (lm *LockManager) ExecuteWithSceneLock(sceneID string, fn func() error) error {
	lock := lm.GetSceneLock(sceneID)
	lock.Lock()
	defer lock.Unlock()

	return fn()
}

// ExecuteWithSceneReadLock 在场景读锁保护下执行操作
func (lm *LockManager) ExecuteWithSceneReadLock(sceneID string, fn func() error) error {
	lock := lm.GetSceneLock(sceneID)
	lock.RLock()
	defer lock.RUnlock()

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

	// 简单有效的清理策略
	const maxLocks = 200 // 根据实际场景数量调整

	// 只有在锁数量过多时才清理
	if len(lm.sceneLocks) > maxLocks {
		// 清空所有锁
		lm.sceneLocks = make(map[string]*sync.RWMutex)
	}
}
