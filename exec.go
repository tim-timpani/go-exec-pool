// Copyright (c) 2023 Timothy Martin
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in all
// copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.

package pool

import (
	"bytes"
	"errors"
	"fmt"
	log "github.com/sirupsen/logrus"
	"math/rand"
	"os/exec"
	"sync"
	"time"
)

type ExecPool struct {
	workerChanIn  chan CommandRequest
	workerChanOut chan CommandResult
	executors     int
	cmdOutput     []CommandResult
	cmdQueue      []CommandRequest
	startTime     time.Time
	endTime       time.Time
	inputClosed   bool
	waitGroup     sync.WaitGroup
	poolId        string
	cmdEnv        []string
}

type CommandRequest struct {
	jobSequence int
	jobId       string
	command     *exec.Cmd
}

type CommandResult struct {
	JobSequence int
	JobId       string
	RunError    error
	StdOut      string
	StdErr      string
	ReturnCode  int
}

func NewExecPool(executors int) *ExecPool {
	return &ExecPool{
		executors:   executors,
		inputClosed: false,
		poolId:      fmt.Sprintf("%s", RandomString(12)),
	}
}

func (e *ExecPool) AddCommand(cmd *exec.Cmd) (id string) {
	seq := len(e.cmdQueue)
	id = fmt.Sprintf("%s-%012d-%s", time.Now().Format(time.RFC3339Nano), seq, e.poolId)
	cmdRequest := CommandRequest{
		jobSequence: seq,
		jobId:       id,
		command:     cmd,
	}
	log.Debugf("job %s queues", id)
	e.cmdQueue = append(e.cmdQueue, cmdRequest)
	return
}

func (e *ExecPool) AddEnv(envSetting string) error {
	if e.inputClosed {
		return errors.New("can not set env after jobs have started")
	}
	e.cmdEnv = append(e.cmdEnv, envSetting)
	return nil
}

func (e *ExecPool) Start() error {

	if e.inputClosed {
		return errors.New("can not start again")
	}
	e.inputClosed = true

	// Create the channels
	e.workerChanIn = make(chan CommandRequest, len(e.cmdQueue))
	e.workerChanOut = make(chan CommandResult, len(e.cmdQueue))

	// Add each command from the queue into the input channel for the workers
	for _, jobRequest := range e.cmdQueue {
		if e.cmdEnv != nil {
			jobRequest.command.Env = e.cmdEnv
		}
		job := CommandRequest{
			jobSequence: jobRequest.jobSequence,
			jobId:       jobRequest.jobId,
			command:     jobRequest.command,
		}
		e.workerChanIn <- job
	}
	close(e.workerChanIn)

	e.startTime = time.Now()

	// Add the workers
	for i := 0; i < e.executors; i++ {
		e.waitGroup.Add(1)
		go e.worker(&e.waitGroup)
	}

	return nil

}

func (e *ExecPool) Wait() error {
	if !e.inputClosed {
		return errors.New("attempting to wait for jobs that have not started")
	}

	// Wait for workers to be done and close the worker channel
	e.waitGroup.Wait()
	close(e.workerChanOut)

	// Save output from the jobs
	for workerOutput := range e.workerChanOut {
		e.cmdOutput = append(e.cmdOutput, workerOutput)
	}

	// Save stats
	e.endTime = time.Now()
	et := e.endTime.Sub(e.startTime)
	log.Debugf("total elapsed time %f seconds", et.Seconds())
	return nil
}

func (e *ExecPool) GetResults(jobId string) *CommandResult {
	for _, output := range e.cmdOutput {
		if output.JobId == jobId {
			return &output
		}
	}
	return nil
}

func (e *ExecPool) worker(wg *sync.WaitGroup) {
	workerId := "worker-" + RandomString(8)
	for job := range e.workerChanIn {
		log.Debugf("%s starting job %s", workerId, job.jobId)
		outBuff := bytes.Buffer{}
		errBuff := bytes.Buffer{}
		job.command.Stdout = &outBuff
		job.command.Stderr = &errBuff
		runErr := job.command.Run()
		output := CommandResult{
			JobId:      job.jobId,
			RunError:   runErr,
			StdOut:     outBuff.String(),
			StdErr:     errBuff.String(),
			ReturnCode: job.command.ProcessState.ExitCode(),
		}
		e.workerChanOut <- output
	}
	wg.Done()
}

func RandomString(n int) string {
	var letters = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789")

	s := make([]rune, n)
	for i := range s {
		s[i] = letters[rand.Intn(len(letters))]
	}
	return string(s)
}
