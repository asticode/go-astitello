package main

import (
	"github.com/asticode/go-astilog"
	"github.com/asticode/go-astitello"
	astiworker "github.com/asticode/go-astitools/worker"
	"github.com/pkg/errors"
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

	// Handle events
	d.On(astitello.TakeOffEvent, func(interface{}) { astilog.Info("main: drone has took off!") })

	// Connect drone
	if err := d.Connect(); err != nil {
		astilog.Error(errors.Wrap(err, "main: connecting to drone failed"))
		return
	}
	defer d.Close()

	// Take off
	if err := d.TakeOff(); err != nil {
		astilog.Error(errors.Wrap(err, "main: taking off failed"))
		return
	}

	// Wait
	w.Wait()

	// Land
	if err := d.Land(); err != nil {
		astilog.Error(errors.Wrap(err, "main: landing failed"))
		return
	}
}
