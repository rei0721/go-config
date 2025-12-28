package main

import (
	"context"
	"fmt"
	"log"
	"time"

	config "github.com/rei0721/go-config"
)

func main() {
	// 创建调度池：2个工作器，队列容量10
	pool := config.NewSchedulerPool(2, 10)

	// 启动调度池
	ctx := context.Background()
	if err := pool.Start(ctx); err != nil {
		log.Fatalf("Failed to start scheduler pool: %v", err)
	}
	defer func() {
		if err := pool.Stop(5 * time.Second); err != nil {
			log.Printf("Error stopping pool: %v", err)
		}
	}()

	fmt.Println("Scheduler Pool started with 2 workers")

	// 提交一些任务
	for i := 0; i < 5; i++ {
		taskID := i
		task := config.NewConfigTask(
			fmt.Sprintf("task-%d", taskID),
			config.TaskTypeHook,
			taskID, // 优先级
			func(ctx context.Context) error {
				fmt.Printf("Executing task %d\n", taskID)
				time.Sleep(100 * time.Millisecond) // 模拟工作
				fmt.Printf("Task %d completed\n", taskID)
				return nil
			},
		)

		if err := pool.Submit(task); err != nil {
			log.Printf("Failed to submit task %d: %v", taskID, err)
		}
	}

	// 使用SubmitFunc提交函数任务
	for i := 0; i < 3; i++ {
		funcID := i
		if err := pool.SubmitFunc(func(ctx context.Context) error {
			fmt.Printf("Executing function task %d\n", funcID)
			time.Sleep(50 * time.Millisecond)
			fmt.Printf("Function task %d completed\n", funcID)
			return nil
		}); err != nil {
			log.Printf("Failed to submit function task %d: %v", funcID, err)
		}
	}

	// 等待任务完成
	time.Sleep(2 * time.Second)

	// 显示统计信息
	metrics := pool.GetDetailedMetrics()
	fmt.Printf("\n=== Pool Statistics ===\n")
	fmt.Printf("Total Submissions: %d\n", metrics.TotalSubmissions)
	fmt.Printf("Completed Tasks: %d\n", metrics.CompletedTasks)
	fmt.Printf("Failed Tasks: %d\n", metrics.FailedTasks)
	fmt.Printf("Success Rate: %.2f%%\n", metrics.SuccessRate*100)
	fmt.Printf("Average Execution Time: %v\n", metrics.AverageExecTime)
	fmt.Printf("Peak Queue Size: %d\n", metrics.PeakQueueSize)
	fmt.Printf("Uptime: %v\n", metrics.Uptime)

	// 健康检查
	health := pool.HealthCheck()
	fmt.Printf("\n=== Health Check ===\n")
	fmt.Printf("Status: %s\n", health["status"])
	fmt.Printf("Queue Utilization: %.2f%%\n", health["queue_utilization"].(float64)*100)
	fmt.Printf("Worker Utilization: %.2f%%\n", health["worker_utilization"].(float64)*100)

	fmt.Println("\nScheduler Pool example completed successfully!")
}
