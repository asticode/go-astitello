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
	d.On(astitello.GoEvent, astitello.GoEventHandler(func(x, y, z, speed int) { astilog.Infof("main: go: %d %d %d %d", x, y, z, speed) }))

	// Connect drone
	if err := d.Connect(); err != nil {
		astilog.Fatal(errors.Wrap(err, "main: connecting to drone failed"))
	}
	defer d.Close()

	// Make sure to stop
	go func() {
		// Wait for context to be done
		<- w.Context().Done()
	}()

	// Take off
	if err := d.TakeOff(); err != nil {
		astilog.Error(errors.Wrap(err, "main: taking off failed"))
		return
	}

	// Make sure to land
	defer func() {
		if err := d.Land(); err != nil {
			astilog.Error(errors.Wrap(err, "main: landing failed"))
			return
		}
	}()

	// Curve
	if err := d.Curve(20, 20, 20, 40, 40, 20, 20); err != nil {
		astilog.Error(errors.Wrap(err, "main: curving failed"))
		return
	}

	// Wait
	w.Wait()
}
