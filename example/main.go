package main

import (
	"fmt"
	"io"
	"log"
	"os/exec"

	"github.com/asticode/go-astikit"
	"github.com/asticode/go-astitello"
)

func main() {
	// Create logger
	l := log.New(log.Writer(), log.Prefix(), log.Flags())

	// Create worker
	w := astikit.NewWorker(astikit.WorkerOptions{Logger: l})

	// Create the drone
	d := astitello.New(l)

	// Handle signals
	w.HandleSignals(astikit.TermSignalHandler(func() {
		// Make sure to land on term signal
		if err := d.Land(); err != nil {
			l.Println(fmt.Errorf("main: landing failed: %w", err))
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
					err = fmt.Errorf("main: piping stdin failed: %w", err)
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
						l.Println(fmt.Errorf("main: writing video packet failed: %w", err))
						return
					}
				}))
				return
			},
			Name: "ffmpeg",
		}); err != nil {
			l.Println(fmt.Errorf("main: executing ffmpeg failed: %w", err))
			return
		}
		defer in.Close()

		// Update
		video = true
	} else {
		// Log
		l.Println("main: ffmpeg was not found, video won't be started")
	}

	// Handle take off event
	d.On(astitello.TakeOffEvent, func(interface{}) { l.Println("main: drone has took off!") })

	// Start the drone
	if err := d.Start(); err != nil {
		l.Println(fmt.Errorf("main: starting to the drone failed: %w", err))
		return
	}
	defer d.Close()

	// Execute in a task
	w.NewTask().Do(func() {
		// Start video
		if video {
			if err := d.StartVideo(); err != nil {
				l.Println(fmt.Errorf("main: starting video failed: %w", err))
				return
			}
		}

		// Take off
		if err := d.TakeOff(); err != nil {
			l.Println(fmt.Errorf("main: taking off failed: %w", err))
			return
		}

		// Flip
		if err := d.Flip(astitello.FlipRight); err != nil {
			l.Println(fmt.Errorf("main: flipping failed: %w", err))
			return
		}

		// Log state
		l.Printf("main: state is: %+v\n", d.State())

		// Land
		if err := d.Land(); err != nil {
			l.Println(fmt.Errorf("main: landing failed: %w", err))
			return
		}

		// Stop video
		if video {
			if err := d.StopVideo(); err != nil {
				l.Println(fmt.Errorf("main: stopping video failed: %w", err))
				return
			}
		}

		// Stop worker
		w.Stop()
	})

	// Wait
	w.Wait()
}
