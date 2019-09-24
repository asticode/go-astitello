package main

import (
	"flag"
	"os/exec"

	"github.com/asticode/go-astilog"
	"github.com/asticode/go-astitello"
	"github.com/pkg/errors"
)

func main() {
	// Set logger
	flag.Parse()
	astilog.SetLogger(astilog.New(astilog.FlagConfig()))

	// Create the drone
	d := astitello.New()

	// Check whether to start video
	var video bool
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		// Log
		astilog.Info("main: ffmpeg was not found, video won't be started")
	} else {
		// Create command
		cmd := exec.Command("ffmpeg", "-y", "-i", "pipe:0", "example.mp4")

		// Pipe stdin
		in, err := cmd.StdinPipe()
		if err != nil {
			astilog.Error(errors.Wrap(err, "main: piping stdin failed"))
			return
		}
		defer in.Close()

		// Handle new video packets
		d.On(astitello.VideoPacketEvent, astitello.VideoPacketEventHandler(func(p []byte) {
			// Write the packet in stdin
			if _, err := in.Write(p); err != nil {
				astilog.Error(errors.Wrap(err, "main: writing video packet failed"))
				return
			}
		}))

		// Run the cmd
		go func() {
			if b, err := cmd.CombinedOutput(); err != nil {
				astilog.Error(errors.Wrapf(err, "main: running cmd failed with output: %s", b))
				return
			}
		}()

		// Update
		video = true
	}

	// Handle take off event
	d.On(astitello.TakeOffEvent, func(interface{}) { astilog.Warn("main: drone has took off!") })

	// Connect to the drone
	if err := d.Connect(); err != nil {
		astilog.Error(errors.Wrap(err, "main: connecting to the drone failed"))
		return
	}
	defer d.Disconnect()

	// Start video
	if video {
		if err := d.StartVideo(); err != nil {
			astilog.Error(errors.Wrap(err, "main: starting video failed"))
			return
		}
	}

	// Take off
	if err := d.TakeOff(); err != nil {
		astilog.Error(errors.Wrap(err, "main: taking off failed"))
		return
	}

	// Flip
	if err := d.Flip(astitello.FlipRight); err != nil {
		astilog.Error(errors.Wrap(err, "main: flipping failed"))
		return
	}

	// Log state
	astilog.Infof("main: state is: %+v", d.State())

	// Land
	if err := d.Land(); err != nil {
		astilog.Error(errors.Wrap(err, "main: landing failed"))
		return
	}

	// Stop video
	if video {
		if err := d.StopVideo(); err != nil {
			astilog.Error(errors.Wrap(err, "main: stopping video failed"))
			return
		}
	}
}
