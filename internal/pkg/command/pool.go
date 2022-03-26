package command

import (
	"fmt"
	"time"

	log "github.com/sirupsen/logrus"
)

type Pool struct {
	commands  chan *Command
	kill      chan struct{}
	noRunners int
	runners   []*runner
}

func NewPool(noRunners int) *Pool {
	runners := []*runner{}
	for i := 0; i < noRunners; i++ {
		runners = append(runners, newRunner(i))
	}
	return &Pool{
		commands:  make(chan *Command),
		kill:      make(chan struct{}),
		noRunners: noRunners,
		runners:   runners,
	}
}

func (p *Pool) Start() {
	log.Debug("Start() called on worker thread pool")
	for _, curRunner := range p.runners {
		curRunner.start()
	}
	go func() {
		lastStatTime := time.Now()
		for {
			select {
			case <-p.kill:
				log.Info("Killing all worker threads")
				for _, curRunner := range p.runners {
					curRunner.stop()
				}
				break
			default:
				curCommand := <-p.commands
				var bestRunner *runner
				var bestRunnerCapacity int64
				log.Debug("Checking which thread is best suited to run command")
				for curRunnerIndex, curRunner := range p.runners {
					if curRunnerIndex == 0 {
						log.Debug("First thread considered the best")
						bestRunner = curRunner
						bestRunnerCapacity = curRunner.getCapacity()
					} else {
						curRunnerCapacity := curRunner.getCapacity()
						log.Debug(fmt.Sprintf(
							"Current worker thread %d(capacity: %d). Best worker thread %d (capacity: %d",
							curRunner.id,
							curRunnerCapacity,
							bestRunner.id,
							bestRunnerCapacity))
						if curRunnerCapacity < bestRunnerCapacity {
							bestRunner = curRunner
							bestRunnerCapacity = curRunnerCapacity
						}
					}
					if time.Now().Sub(lastStatTime).Minutes() > 5 {
						log.Info(fmt.Sprintf("Worker thread %d has executed %d commands so far", curRunner.id, curRunner.gettNoCommandsRun()))
						if curRunnerIndex == (len(p.runners) - 1) {
							lastStatTime = time.Now()
						}
					}
				}

				log.Debug(fmt.Sprintf("Sending command to worker thread with ID %d", bestRunner.getID()))
				bestRunner.addCommand(curCommand)
			}
		}
	}()
}

func (p *Pool) AddCommand(c *Command) {
	p.commands <- c
}

func (p *Pool) Stop() {
	log.Debug("Stop() called on worker thread pool")
	p.kill <- struct{}{}
}
