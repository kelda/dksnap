package main

import (
	"fmt"
	"io"
	"time"
)

// ProgressPrinter periodically prints until it is stopped.
type ProgressPrinter struct {
	out     io.Writer
	stop    chan struct{}
	stopped chan struct{}
}

// NewProgressPrinter returns a new ProgressPrinter.
func NewProgressPrinter(out io.Writer) *ProgressPrinter {
	return &ProgressPrinter{
		out:     out,
		stop:    make(chan struct{}),
		stopped: make(chan struct{}),
	}
}

// Start starts the printer, and immediately returns.
func (pp *ProgressPrinter) Start() {
	go func() {
		defer close(pp.stopped)
		for {
			ticker := time.NewTicker(1 * time.Second)
			select {
			case <-pp.stop:
				fmt.Fprintln(pp.out)
				return
			case <-ticker.C:
				fmt.Fprintf(pp.out, ".")
			}
		}
	}()
}

// Stop stops printing to `out`. No prints will occur after `Stop` returns.
func (pp *ProgressPrinter) Stop() {
	close(pp.stop)
	<-pp.stopped
}
