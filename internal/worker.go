package internal

import (
	"fmt"
	"net/url"
	"sync"
)

type Worker struct {
	Client Client
	Jobs   <-chan Job
	Result chan<- Result
	Error  chan error
	Robots *Robots
	Wg     *sync.WaitGroup
}

func NewWorker(client Client, jobs <-chan Job, result chan<- Result, error chan error, robots *Robots, wg *sync.WaitGroup) *Worker {
	return &Worker{
		Client: client,
		Jobs:   jobs,
		Result: result,
		Error:  error,
		Robots: robots,
		Wg:     wg,
	}
}

func (worker *Worker) Run(id int) {
	defer worker.Wg.Done()

	for {
		select {
		case <-worker.Client.Context.ctx.Done():
			worker.Error <- worker.Client.Context.ctx.Err()
			worker.Client.Context.cancel()
			return
		case job, ok := <-worker.Jobs:
			if !ok {
				error := fmt.Errorf("there is no initial link that can be processed")
				worker.Error <- error
				return
			}

			parsedURL, err := url.Parse(job.URL)

			if err != nil {
				error := fmt.Errorf("parsing url: %w", err)
				worker.Error <- error
				return
			}

			ticker := worker.Robots.RateLimit(worker.Client.BotName, parsedURL)

			<-ticker.C

			doc, err := FetchPage(worker.Client.Context.ctx, worker.Client.Client, worker.Client.AgentName, parsedURL)

			if err != nil {
				error := err
				worker.Error <- error
				return
			}

			fmt.Println("----------------------------------------")
			fmt.Printf("Worker %d\nProcessing %v\nCurrent Layer: %d\n", id, parsedURL.String(), job.CurrentDepth+1)
			fmt.Println("----------------------------------------")
			res := Result{
				JobID:    job.ID,
				StartURL: parsedURL,
				Finding:  make([]string, 0),
			}

			TraverseNode(doc, &res)

			worker.Result <- res
		}
	}
}
