package main

import (
	"fmt"
	"io"
	"time"
)

type ProgressPrinter struct {
	out     io.Writer
	stop    chan struct{}
	stopped chan struct{}
}

func NewProgressPrinter(out io.Writer) *ProgressPrinter {
	return &ProgressPrinter{
		out:     out,
		stop:    make(chan struct{}),
		stopped: make(chan struct{}),
	}
}

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

func (pp *ProgressPrinter) Stop() {
	close(pp.stop)
	<-pp.stopped
}
