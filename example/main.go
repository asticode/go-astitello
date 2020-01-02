package main

import (
	"flag"
	"io"
	"os/exec"

	"github.com/asticode/go-astikit"
	"github.com/asticode/go-astilog"
	"github.com/asticode/go-astitello"
	"github.com/pkg/errors"
)

func main() {
	// Set logger
	flag.Parse()
	astilog.SetLogger(astilog.New(astilog.FlagConfig()))

	// Create worker
	w := astikit.NewWorker(astikit.WorkerOptions{Logger: astilog.GetLogger()})

	// Create the drone
	d := astitello.New()

	// Handle signals
	w.HandleSignals(astikit.TermSignalHandler(func() {
		// Make sure to land on term signal
		if err := d.Land(); err != nil {
			astilog.Error(errors.Wrap(err, "main: landing failed"))
			return
		}
	}))

	// Check whether ffmpeg exists on the machine
	var video bool
	if _, err := exec.LookPath("ffmpeg"); err == nil {
		// Execute ffmpeg
		var in io.WriteCloser
		if _, err = astikit.ExecCmd(w, astikit.ExecCmdOptions{
			Args: []string{"-y", "-i", "pipe:0", "example.ts"},
			CmdAdapter: func(cmd *exec.Cmd, h *astikit.ExecHandler) (err error) {
				// Pipe stdin
				if in, err = cmd.StdinPipe(); err != nil {
					err = errors.Wrap(err, "main: piping stdin failed")
					return
				}

				// Handle new video packets
				d.On(astitello.VideoPacketEvent, astitello.VideoPacketEventHandler(func(p []byte) {
					// Check status
					if h.Status() != astikit.ExecStatusRunning {
						return
					}

					// Write the packet in stdin
					if _, err := in.Write(p); err != nil {
						astilog.Error(errors.Wrap(err, "main: writing video packet failed"))
						return
					}
				}))
				return
			},
			Name: "ffmpeg",
		}); err != nil {
			astilog.Error(errors.Wrap(err, "main: executing ffmpeg failed"))
			return
		}
		defer in.Close()

		// Update
		video = true
	} else {
		// Log
		astilog.Info("main: ffmpeg was not found, video won't be started")
	}

	// Handle take off event
	d.On(astitello.TakeOffEvent, func(interface{}) { astilog.Warn("main: drone has took off!") })

	// Start the drone
	if err := d.Start(); err != nil {
		astilog.Error(errors.Wrap(err, "main: starting to the drone failed"))
		return
	}
	defer d.Close()

	// Execute in a task
	w.NewTask().Do(func() {
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

		// Stop worker
		w.Stop()
	})

	// Wait
	w.Wait()
}
