package scheduler

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/robfig/cron/v3"
)

var instance *CronManager

func init() {
	instance = &CronManager{
		cronInstance: newCron(),
		jobs:         make(map[int]*CronJob),
		jobCounter:   0,
	}
}

// newCron builds the cron scheduler with a panic-recovering job chain.
// robfig/cron v3 does NOT recover panics by default: a single panicking job
// (GeoIP update, alert check, ...) would otherwise crash the whole server process.
func newCron() *cron.Cron {
	return cron.New(
		cron.WithSeconds(),
		cron.WithLocation(time.UTC),
		cron.WithChain(cron.Recover(slogCronLogger{})),
	)
}

// slogCronLogger adapts cron.Logger to slog.
type slogCronLogger struct{}

func (slogCronLogger) Info(msg string, kv ...interface{}) { slog.Info("cron: "+msg, kv...) }
func (slogCronLogger) Error(err error, msg string, kv ...interface{}) {
	slog.Error("cron: "+msg, append([]interface{}{"error", err}, kv...)...)
}

// JobLocker 尝试为一次任务执行获取跨实例互斥锁。
// 返回 acquired=false 表示其他实例正在执行该任务，本实例应跳过本轮。
// release 在任务结束后调用（acquired=true 时非 nil）。
type JobLocker func(ctx context.Context, jobName string) (release func(), acquired bool)

var (
	lockerMu  sync.RWMutex
	jobLocker JobLocker
)

// SetJobLocker 安装分布式锁提供者（通常为 PostgreSQL advisory lock）。
// 未安装时任务直接执行——单实例部署无需锁。
func SetJobLocker(l JobLocker) {
	lockerMu.Lock()
	defer lockerMu.Unlock()
	jobLocker = l
}

func currentLocker() JobLocker {
	lockerMu.RLock()
	defer lockerMu.RUnlock()
	return jobLocker
}

type JobFunction func(params any)

// CronJob defines a scheduled job
type CronJob struct {
	ID      int          // Job ID, used for unique identification and management
	Spec    string       // Cron expression
	JobFunc JobFunction  // Function to execute
	Params  *any         // Store task parameters
	EntryID cron.EntryID // Cron library Entry ID, used for job removal
}

// CronManager is the cron job manager
type CronManager struct {
	cronInstance *cron.Cron       // Cron scheduler instance
	jobs         map[int]*CronJob // Stores registered jobs, keyed by job ID
	jobCounter   int              // Job ID counter
	mu           sync.Mutex       // Mutex for concurrent-safe access to jobs map
}

func GetCronManager() *CronManager {
	return instance
}

// Start Start the cron scheduler
func (cm *CronManager) Start() {
	cm.cronInstance.Start()
	slog.Info("cron: scheduler started")
}

// Stop Stop the cron scheduler
func (cm *CronManager) Stop() {
	cm.cronInstance.Stop()
	slog.Info("cron: scheduler stopped")
}

// AddJob Dynamically add a scheduled job
func (cm *CronManager) AddJob(spec string, jobFunc JobFunction, params any) (int, error) {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	// 任务名由注册顺序 + spec 决定；同一二进制在所有副本上注册顺序一致，
	// 因此可直接作为跨实例锁的 key。
	jobID := cm.jobCounter + 1
	jobName := fmt.Sprintf("cron:%d:%s", jobID, spec)

	wrappedJobFunc := func() {
		if locker := currentLocker(); locker != nil {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			release, acquired := locker(ctx, jobName)
			cancel()
			if !acquired {
				slog.Debug("cron: job skipped, lock held by another instance", "job", jobName)
				return
			}
			defer release()
		}
		jobFunc(params)
	}

	entryID, err := cm.cronInstance.AddFunc(spec, wrappedJobFunc)
	if err != nil {
		return 0, fmt.Errorf("failed to add job to cron: %w", err)
	}

	cm.jobCounter = jobID

	cronJob := &CronJob{
		ID:      jobID,
		Spec:    spec,
		JobFunc: jobFunc,
		Params:  &params, // 存储参数
		EntryID: entryID,
	}
	cm.jobs[jobID] = cronJob // Store job info in the map

	slog.Info("cron: job added", "job_id", jobID, "spec", spec)

	return jobID, nil
}

// RemoveJob Dynamically remove a scheduled job
func (cm *CronManager) RemoveJob(jobID int) error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	cronJob, exists := cm.jobs[jobID]
	if !exists {
		return fmt.Errorf("job with ID %d not found", jobID)
	}

	cm.cronInstance.Remove(cronJob.EntryID) // Remove job from cron scheduler
	delete(cm.jobs, jobID)                  // Delete job info from jobs map

	slog.Info("cron: job removed", "job_id", jobID)
	return nil
}

// ListJobs List all registered scheduled jobs
func (cm *CronManager) ListJobs() []*CronJob {
	cm.mu.Lock()         // Lock for concurrent-safe jobs map access
	defer cm.mu.Unlock() // Unlock on function exit

	jobList := make([]*CronJob, 0, len(cm.jobs))
	for _, job := range cm.jobs {
		jobList = append(jobList, job)
	}

	return jobList
}
