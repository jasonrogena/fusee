package command

import (
	"fmt"
	"sync"
	"time"

	log "github.com/sirupsen/logrus"
)

type runner struct {
	commands              chan *Command
	kill                  chan struct{}
	curCmdStartTime       time.Time
	curCmdStartTimeMutex  *sync.Mutex
	id                    int
	isRunningCommand      bool
	isRunningCommandMutex *sync.Mutex
	noCommandsRun         int
	noCommandsRunMutex    *sync.Mutex
}

func newRunner(id int) *runner {
	return &runner{
		commands:              make(chan *Command),
		kill:                  make(chan struct{}),
		curCmdStartTimeMutex:  new(sync.Mutex),
		id:                    id,
		isRunningCommand:      false,
		isRunningCommandMutex: new(sync.Mutex),
		noCommandsRun:         0,
		noCommandsRunMutex:    new(sync.Mutex),
	}
}

func (r *runner) resetCurrentCommandStartTime() {
	r.curCmdStartTimeMutex.Lock()
	defer r.curCmdStartTimeMutex.Unlock()
	r.curCmdStartTime = time.Now()
}

func (r *runner) getCurrentCommandStartTime() time.Time {
	r.curCmdStartTimeMutex.Lock()
	defer r.curCmdStartTimeMutex.Unlock()
	tm := r.curCmdStartTime
	return tm
}

func (r *runner) setIsRunningCommand(status bool) {
	r.isRunningCommandMutex.Lock()
	defer r.isRunningCommandMutex.Unlock()
	r.isRunningCommand = status
}

func (r *runner) getIsRunningCommand() bool {
	r.isRunningCommandMutex.Lock()
	defer r.isRunningCommandMutex.Unlock()
	status := r.isRunningCommand
	return status
}

func (r *runner) incrementNoCommandsRun() {
	r.noCommandsRunMutex.Lock()
	defer r.noCommandsRunMutex.Unlock()
	r.noCommandsRun++
}

func (r *runner) gettNoCommandsRun() int {
	r.noCommandsRunMutex.Lock()
	defer r.noCommandsRunMutex.Unlock()
	noCommands := r.noCommandsRun
	return noCommands
}

func (r *runner) getCapacity() int64 {
	var timeDiff int64
	timeDiff = 0
	curCmdStartTime := r.getCurrentCommandStartTime()
	if !curCmdStartTime.IsZero() {
		timeDiff = time.Now().Sub(curCmdStartTime).Nanoseconds()
	}
	if timeDiff < 0 {
		// Assume system time was changed between when the last command started running and now
		timeDiff = 0
		log.Warn("Current running command in goroutine appears to have started in the future. Assuming system time has been changed")
	}
	noCommands := len(r.commands)
	if r.getIsRunningCommand() {
		noCommands++
	}

	return (int64(noCommands) * timeDiff) + int64(noCommands)
}

func (r *runner) start() {
	log.Debug(fmt.Sprintf("start() called on worker thread %d", r.id))
	go func() {
		for {
			select {
			case <-r.kill:
				log.Info(fmt.Sprintf("Stopping execution of worker thread %d", r.id))
				break
			default:
				curCommand := <-r.commands
				r.setIsRunningCommand(true)
				r.incrementNoCommandsRun()
				r.resetCurrentCommandStartTime()
				curCommand.Run()
				r.setIsRunningCommand(false)
			}
		}
	}()
}

func (r *runner) stop() {
	log.Debug(fmt.Sprintf("stop() called on worker thread %d", r.id))
	r.kill <- struct{}{}
}

func (r *runner) addCommand(c *Command) {
	log.Debug(fmt.Sprintf("addCommand called for worker thread %d", r.id))
	r.commands <- c
}

func (r *runner) getID() int {
	return r.id
}
