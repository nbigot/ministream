package cron

import (
	"errors"
	"ministream/log"
	"sync"
	"time"

	"go.uber.org/zap"
)

type CronJobStatus int64

const (
	CronJobStopped  = 0
	CronJobStarting = 1
	CronJobRunning  = 2
	CronJobStopping = 3
)

type CronJobHandler interface {
	Exec(eventTime *time.Time, c *CronJob) error
}

type CronJob struct {
	frequencySec int
	handler      CronJobHandler
	mu           sync.Mutex
	muFeed       sync.Mutex
	// context
	//ch          chan struct
	chRequests   chan time.Time
	chBufferSize int
	lastRequest  time.Time
	ticker       *time.Ticker
	status       CronJobStatus
}

func MakeCronJob(frequencySec int, chBufferSize int, handler CronJobHandler) *CronJob {
	c := CronJob{
		status:       CronJobStopped,
		frequencySec: frequencySec,
		chBufferSize: chBufferSize,
		lastRequest:  time.Time{},
		ticker:       nil,
		handler:      handler,
		chRequests:   nil,
	}
	return &c
}

func cronJobExec(c *CronJob) {
	for eventTime := range c.ticker.C {
		if c.status == CronJobRunning {
			func() {
				c.mu.Lock()

				defer func() {
					if r := recover(); r != nil {
						var err error
						switch x := r.(type) {
						case string:
							err = errors.New(x)
						case error:
							err = x
						default:
							err = errors.New("unknown panic")
						}
						log.Logger.Error("cronJobExec recovered from panic", zap.Error(err))
					}
					c.mu.Unlock()
				}()

				for {
					select {
					case <-c.chRequests:
						if err := c.handler.Exec(&eventTime, c); err != nil {
							log.Logger.Error("cronJobExec error", zap.Error(err))
						}
						c.lastRequest = eventTime
					default:
						return
					}
				}
			}()
		}
	}
}

func (c *CronJob) Start() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.status != CronJobStopped {
		return errors.New("can't start cronjob")
	}

	c.status = CronJobStarting
	c.chRequests = make(chan time.Time, c.chBufferSize)
	c.ticker = time.NewTicker(time.Duration(c.frequencySec) * time.Second)
	go cronJobExec(c)
	c.status = CronJobRunning
	return nil
}

func (c *CronJob) Stop() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.status != CronJobRunning {
		return errors.New("can't stop cronjob")
	}

	c.status = CronJobStopping
	close(c.chRequests)
	c.ticker.Stop()

	// search for any remain request event
	lastRequestFound := time.Time{}
	for evRequest := range c.chRequests {
		lastRequestFound = evRequest
	}
	if !lastRequestFound.IsZero() {
		// process one last remain request
		c.handler.Exec(&lastRequestFound, c)
		c.lastRequest = lastRequestFound
	}

	c.status = CronJobStopped
	return nil
}

func (c *CronJob) GetStatus() CronJobStatus {
	return c.status
}

func (c *CronJob) SendRequest(squashQueuedRequests bool) error {
	c.muFeed.Lock()
	defer c.muFeed.Unlock()

	if c.status != CronJobRunning {
		return errors.New("can't send request")
	}

	if squashQueuedRequests {
		c.ClearRequests()
	} else {
		if len(c.chRequests) == c.chBufferSize {
			return errors.New("channel buffer is full")
		}
	}

	c.chRequests <- time.Now() // TODO? bloquant?
	return nil
}

func (c *CronJob) ClearRequests() {
	// remove all requests from the channel
	for {
		select {
		case <-c.chRequests:
		default:
			return
		}
	}
}
