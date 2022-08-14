// Cron job rate limiter
// note: RL eq Rate Limiter

package cron

import (
	"errors"
	"ministream/log"
	"sync"
	"time"

	"go.uber.org/zap"
)

type CronJobRLStatus int64

const (
	CronJobRLStopped  = 0
	CronJobRLStarting = 1
	CronJobRLRunning  = 2
	CronJobRLStopping = 3
)

type CronJobRLHandler interface {
	Exec(c *CronJobRL) error
}

type CronJobRL struct {
	frequencySec             int
	handler                  CronJobRLHandler
	mu                       sync.Mutex
	muFeed                   sync.Mutex
	chRequests               chan time.Time
	reqFlag                  bool
	lastTimeProcessedRequest time.Time
	//ticker                   *time.Ticker
	timer  *time.Timer
	status CronJobRLStatus
}

func MakeCronJobRateLimiter(frequencySec int, handler CronJobRLHandler) *CronJobRL {
	c := CronJobRL{
		status:                   CronJobRLStopped,
		frequencySec:             frequencySec,
		lastTimeProcessedRequest: time.Time{},
		//ticker:                   nil,
		timer:      nil,
		handler:    handler,
		chRequests: nil,
		reqFlag:    false,
	}
	return &c
}

func (c *CronJobRL) cronJobExecRL() {
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
			log.Logger.Error("cronJobExecRL recovered from panic", zap.Error(err))
		}
		c.mu.Unlock()
	}()

	if err := c.handler.Exec(c); err != nil {
		log.Logger.Error("cronJobExecRL error", zap.Error(err))
	}
	c.lastTimeProcessedRequest = time.Now()
}

func cronJobRLBGLoop(c *CronJobRL) {
	for {
		select {
		case <-c.chRequests:
			c.cronJobExecRL()
		default:
			return
		}
	}
}

func (c *CronJobRL) Start() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.status != CronJobRLStopped {
		return errors.New("can't start cronjob")
	}

	c.status = CronJobRLStarting
	c.chRequests = make(chan time.Time, 1)
	//c.ticker = time.NewTicker(time.Duration(c.frequencySec) * time.Second)
	go cronJobRLBGLoop(c)
	c.status = CronJobRLRunning
	return nil
}

func (c *CronJobRL) Stop() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.status != CronJobRLRunning {
		return errors.New("can't stop cronjob")
	}

	c.status = CronJobRLStopping
	if c.timer != nil {
		c.timer.Stop()
	}
	//c.ticker.Stop()
	close(c.chRequests)
	//c.reqFlag = False

	// search for any remain request event
	lastRequestFound := time.Time{}
	for evRequest := range c.chRequests {
		lastRequestFound = evRequest
	}
	if !lastRequestFound.IsZero() {
		// process one last remain request
		//c.handler.Exec(&lastRequestFound, c)
		c.handler.Exec(c)
		//c.lastRequest = lastRequestFound
	}

	c.status = CronJobRLStopped
	return nil
}

func (c *CronJobRL) GetStatus() CronJobRLStatus {
	return c.status
}

func (c *CronJobRL) SendRequest() error {
	// async send request
	c.muFeed.Lock()
	defer c.muFeed.Unlock()

	if c.status != CronJobRLRunning {
		return errors.New("can't send request")
	}

	if len(c.chRequests) > 0 {
		// there is already a job pending
		return nil
	}

	if c.timer == nil {
		// c.lastTimeProcessedRequest
		//c.timer = time.NewTimer(time.Second * c.frequencySec)
		c.timer = time.AfterFunc(
			time.Duration(c.frequencySec)*time.Second,
			func() {
				if c.status == CronJobRLRunning {
					c.chRequests <- time.Now() // TODO? bloquant?
				}
				c.timer = nil
			},
		)
	}

	// if c.reqFlag {
	// 	// there is already a request pending
	// 	return nil
	// }
	// c.reqFlag = True
	// c.chRequests <- time.Now() // TODO? bloquant?

	return nil
}

// func (c *CronJobRL) ClearRequests() {
// 	// remove all requests from the channel
// 	for {
// 		select {
// 		case <-c.chRequests:
// 		default:
// 			return
// 		}
// 	}
// }
