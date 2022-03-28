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
		commandIndex := -1
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
				commandIndex++
				// try and see if the next round-robbed worker is available
				designatedRunner := p.runners[commandIndex%len(p.runners)]
				designatedRunner.addCommand(curCommand)

				if time.Now().Sub(lastStatTime).Minutes() > 5 {
					for curRunnerIndex, curRunner := range p.runners {
						log.Info(fmt.Sprintf("Worker thread %d has executed %d commands so far", curRunner.id, curRunner.gettNoCommandsRun()))
						if curRunnerIndex == (len(p.runners) - 1) {
							lastStatTime = time.Now()
						}
					}
				}
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
