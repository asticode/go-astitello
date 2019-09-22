package main

import (
	"github.com/asticode/go-astilog"
	"github.com/asticode/go-astitello"
	astiworker "github.com/asticode/go-astitools/worker"
)

func main() {
	// Set logger
	astilog.SetDefaultLogger()

	// Create worker
	w := astiworker.NewWorker()

	// Handle signals
	w.HandleSignals()

	// Create drone
	d := astitello.New()

	d.On(astitello.StateEvent, astitello.StateEventHandler(func(s astitello.State) { astilog.Warnf("state: %+v", s) }))

	// Create task
	t := w.NewTask()
	go func() {
		// Make sure task is stopped
		defer t.Done()

		// Start drone
		d.Start(w.Context())
		defer d.Close()


		// Wait for context to be done
		<-w.Context().Done()
	}()

	// Wait
	w.Wait()
}
