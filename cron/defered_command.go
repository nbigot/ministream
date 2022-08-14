// Defered command
// inspired by https://github.com/elastic/beats/blob/master/filebeat/registrar/registrar.go

package cron

import (
	"ministream/log"
	"sync"
	"time"

	"go.uber.org/zap"
)

type TargetController interface {
	OnRegisterDeferedCommand() error
	OnStartDeferedCommand() error
	OnStopDeferedCommand() error
	OnRequestDeferedCommand(cmd interface{}) error
	OnExecDeferedCommand() error
}

type DeferedCommand struct {
	log *zap.Logger

	// registrar event input and output
	//Channel              chan []file.State
	Channel chan []interface{}
	//out                  successLogger
	bufferedStateUpdates int

	// shutdown handling
	done chan struct{}
	wg   sync.WaitGroup

	// state storage
	//states       *file.States      // Map with all file paths inside and the corresponding state
	//store        *statestore.Store // Store keeps states in memory and on disk
	ctrl TargetController

	flushTimeout time.Duration
}

// New creates a new Registrar instance, updating the registry file on
// `file.State` updates. New fails if the file can not be opened or created.
func New(logger *zap.Logger, ctrl TargetController, flushTimeout time.Duration) (*DeferedCommand, error) {
	//store, err := stateStore.Access()
	err := ctrl.OnRegisterDeferedCommand()
	if err != nil {
		return nil, err
	}

	d := &DeferedCommand{
		log:     logger,
		Channel: make(chan []interface{}, 1),
		//out:          out,
		done: make(chan struct{}),
		wg:   sync.WaitGroup{},
		//states:       file.NewStates(),
		//store:        store,
		ctrl:         ctrl,
		flushTimeout: flushTimeout,
	}
	return d, nil
}

func (d *DeferedCommand) Start() error {
	// Load the previous log file locations now, for use in input
	//err := d.loadStates()
	err := d.ctrl.OnStartDeferedCommand()
	if err != nil {
		log.Logger.Error("error loading state", zap.Error(err))
		return err
	}

	d.wg.Add(1)
	go func() {
		defer d.wg.Done()
		d.Run()
	}()

	return nil
}

// Stop stops the DeferedCommand. It waits until Run function finished.
func (d *DeferedCommand) Stop() {
	d.log.Info("Stopping DeferedCommand")
	defer d.log.Info("DeferedCommand stopped")

	close(d.done)
	d.wg.Wait()
}

func (d *DeferedCommand) Run() {
	d.log.Debug("Starting DeferedCommand")
	defer d.log.Debug("Stopping DeferedCommand")

	//defer d.store.Close()
	defer d.ctrl.OnStopDeferedCommand()

	// defer func() {
	// 	writeStates(d.store, d.states.GetStates())
	// }()

	var (
		timer  *time.Timer
		flushC <-chan time.Time

		immediateExecCh chan []interface{}
		deferedExecCh   chan []interface{}
	)

	if d.flushTimeout <= 0 {
		immediateExecCh = d.Channel
	} else {
		deferedExecCh = d.Channel
	}

	for {
		select {
		case <-d.done:
			d.log.Info("Ending DeferedCommand")
			return

		case command := <-immediateExecCh:
			// no flush timeout configured. Immediatly execute command
			//d.onEvents(states)
			//d.commitStateUpdates()
			d.ctrl.OnRequestDeferedCommand(command)
			d.ctrl.OnExecDeferedCommand()

		case command := <-deferedExecCh:
			// flush timeout configured. Only update internal state and track pending
			// updates to be written to registry.
			//d.onEvents(states)
			//d.gcStates()
			d.ctrl.OnRequestDeferedCommand(command)
			if flushC == nil {
				timer = time.NewTimer(d.flushTimeout)
				flushC = timer.C
			}

		case <-flushC:
			timer.Stop()
			//d.commitStateUpdates()
			d.ctrl.OnExecDeferedCommand()

			flushC = nil
			timer = nil
		}
	}
}
