package main

import (
	"github.com/asticode/go-astilog"
	"github.com/asticode/go-astitello"
	"github.com/pkg/errors"
)

func main() {
	// Set logger
	astilog.SetDefaultLogger()

	// Create drone
	d := astitello.New()

	// Handle events
	d.On(astitello.TakeOffEvent, func(interface{}) { astilog.Warn("main: drone has took off!") })

	// Connect drone
	if err := d.Connect(); err != nil {
		astilog.Fatal(errors.Wrap(err, "main: connecting to drone failed"))
	}
	defer d.Disconnect()

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

	// Flip
	if err := d.Flip(astitello.FlipRight); err != nil {
		astilog.Error(errors.Wrap(err, "main: flipping failed"))
		return
	}

	// Log state
	astilog.Infof("main: state is: %+v", d.State())
}
