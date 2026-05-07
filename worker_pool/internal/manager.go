package internal

import (
	"fmt"
	"net/url"
)

type Manager struct {
	Client    Client
	Jobs      chan Job
	Result    <-chan Result
	Error     chan error
	Robots    *Robots
	Scheduler *Scheduler
}

func NewManager(client Client, jobs chan Job, result <-chan Result, error chan error, robots *Robots) *Manager {
	scheduler := &Scheduler{
		Visited: make(map[string]struct{}),
	}
	return &Manager{
		Client:    client,
		Jobs:      jobs,
		Result:    result,
		Error:     error,
		Robots:    robots,
		Scheduler: scheduler,
	}
}

func (man *Manager) Run(startURL string, maxCrawlDepth int) {
	var workerQueue []Job
	activeWorker := 0
	allowed, err := man.Robots.IsAllowed(man.Client.BotName, startURL)

	if err != nil {
		error := err
		man.Error <- error
	}

	if allowed {
		if man.Scheduler.ShouldCrawl(startURL) {
			initialJob := Job{
				ID:           0,
				URL:          startURL,
				CurrentDepth: 0,
			}
			workerQueue = append(workerQueue, initialJob)
			fmt.Println("Initial link sent")
		}
	}

	for {
		var activeJobs chan Job
		var nextJob Job

		if len(workerQueue) > 0 {
			activeJobs = man.Jobs
			nextJob = workerQueue[0]
		}
		MetricQueueDepthSet(len(workerQueue))

		if nextJob.CurrentDepth != maxCrawlDepth {
			select {
			case activeJobs <- nextJob:
				workerQueue = workerQueue[1:]
				activeWorker++
				MetricActiveWorkersInc()
			case links := <-man.Result:
				activeWorker--
				MetricActiveWorkersDec()
				if links.CurrentDepth >= maxCrawlDepth {
					continue
				}

				for _, link := range links.Finding {
					parsedURL, err := url.Parse(link)

					if err != nil {
						error := err
						man.Error <- error
						continue
					}

					if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
						continue
					}

					allowed, err := man.Robots.IsAllowed(man.Client.BotName, link)

					if err != nil {
						error := err
						man.Error <- error
						continue
					}

					if allowed {
						MetricURLsAllowedInc()
						if man.Scheduler.ShouldCrawl(link) {
							newJob := Job{
								ID:           links.JobID + 1,
								URL:          link,
								CurrentDepth: links.CurrentDepth + 1,
							}
							workerQueue = append(workerQueue, newJob)
						}
					}
					if !allowed {
						MetricRobotsDeniedInc()
					}

				}

			}
		} else {
			workerQueue = []Job{}
			activeWorker = 0
		}

		if len(workerQueue) == 0 && activeWorker == 0 {
			close(man.Jobs)
			man.Error <- nil
			return
		}
	}
}
