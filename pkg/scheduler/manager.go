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
	log.Println("Cron scheduler started")
}

// Stop Stop the cron scheduler
func (cm *CronManager) Stop() {
	cm.cronInstance.Stop()
	log.Println("Cron scheduler stopped")
}

// AddJob Dynamically add a scheduled job
func (cm *CronManager) AddJob(spec string, jobFunc JobFunction, params any) (int, error) {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	wrappedJobFunc := func() {
		jobFunc(params)
	}

	entryID, err := cm.cronInstance.AddFunc(spec, wrappedJobFunc)
	if err != nil {
		return 0, fmt.Errorf("failed to add job to cron: %w", err)
	}

	cm.jobCounter++
	jobID := cm.jobCounter

	cronJob := &CronJob{
		ID:      jobID,
		Spec:    spec,
		JobFunc: jobFunc,
		Params:  &params, // 存储参数
		EntryID: entryID,
	}
	cm.jobs[jobID] = cronJob // Store job info in the map

	log.Printf("Job %d added with spec: %s, params: %+v\n", jobID, spec, params)

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

	log.Printf("Job %d removed\n", jobID)
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
