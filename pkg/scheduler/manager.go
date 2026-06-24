package scheduler

import (
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/robfig/cron/v3"
)

var instance *CronManager

func init() {
	instance = &CronManager{
		cronInstance: cron.New(cron.WithSeconds(), cron.WithLocation(time.UTC)),
		jobs:         make(map[int]*CronJob),
		jobCounter:   0,
	}
}

type JobFunction func(params any)

// CronJob 定义一个定时任务的结构
type CronJob struct {
	ID      int          // 任务ID，用于唯一标识和管理任务
	Spec    string       // Cron 表达式
	JobFunc JobFunction  // 要执行的函数
	Params  *any         // 新增字段：存储任务参数
	EntryID cron.EntryID // cron 库返回的任务 Entry ID，用于后续删除任务
}

// CronManager 定时任务管理器
type CronManager struct {
	cronInstance *cron.Cron       // cron 调度器实例
	jobs         map[int]*CronJob // 存储已注册的任务，使用任务ID作为键
	jobCounter   int              // 任务 ID 计数器
	mu           sync.Mutex       // 互斥锁，保护 jobs map 的并发安全
}

func GetCronManager() *CronManager {
	return instance
}

// Start 启动 cron 调度器
func (cm *CronManager) Start() {
	cm.cronInstance.Start()
	log.Println("Cron scheduler started")
}

// Stop 停止 cron 调度器
func (cm *CronManager) Stop() {
	cm.cronInstance.Stop()
	log.Println("Cron scheduler stopped")
}

// AddJob 动态添加一个定时任务
func (cm *CronManager) AddJob(spec string, jobFunc JobFunction, params any) (int, error) {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	cm.jobCounter++
	jobID := cm.jobCounter

	wrappedJobFunc := func() {
		jobFunc(params)
	}

	entryID, err := cm.cronInstance.AddFunc(spec, wrappedJobFunc)
	if err != nil {
		return 0, fmt.Errorf("failed to add job to cron: %w", err)
	}

	cronJob := &CronJob{
		ID:      jobID,
		Spec:    spec,
		JobFunc: jobFunc,
		Params:  &params, // 存储参数
		EntryID: entryID,
	}
	cm.jobs[jobID] = cronJob // 存储任务信息到 map 中

	log.Printf("Job %d added with spec: %s, params: %+v\n", jobID, spec, params)

	return jobID, nil
}

// RemoveJob 动态移除一个定时任务
func (cm *CronManager) RemoveJob(jobID int) error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	cronJob, exists := cm.jobs[jobID]
	if !exists {
		return fmt.Errorf("job with ID %d not found", jobID)
	}

	cm.cronInstance.Remove(cronJob.EntryID) // 从 cron 调度器中移除任务
	delete(cm.jobs, jobID)                  // 从 jobs map 中删除任务信息

	log.Printf("Job %d removed\n", jobID)
	return nil
}

// ListJobs 列出所有已注册的定时任务
func (cm *CronManager) ListJobs() []*CronJob {
	cm.mu.Lock()         // 加锁，保护 jobs map 的并发访问
	defer cm.mu.Unlock() // 函数退出时解锁

	jobList := make([]*CronJob, 0, len(cm.jobs))
	for _, job := range cm.jobs {
		jobList = append(jobList, job)
	}

	return jobList
}
