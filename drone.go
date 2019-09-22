package astitello

import (
	"context"
	"fmt"
	"net"
	"strings"
	"sync"

	"github.com/asticode/go-astilog"
	astievent "github.com/asticode/go-astitools/event"
	"github.com/pkg/errors"
)

// Events
const (
	StateEvent = "state"
)

var ErrNotStarted = errors.New("astitello: not started")

type Drone struct {
	cancel    context.CancelFunc
	cmdConn   *net.UDPConn
	ctx       context.Context
	d         *astievent.Dispatcher
	lr        string
	mc        *sync.Mutex // Locks sendCmd
	oc        *sync.Once
	os        *sync.Once
	rc        *sync.Cond
	stateConn *net.UDPConn
}

func New() *Drone {
	return &Drone{
		d:  astievent.NewDispatcher(),
		mc: &sync.Mutex{},
		oc: &sync.Once{},
		os: &sync.Once{},
		rc: sync.NewCond(&sync.Mutex{}),
	}
}

func (d *Drone) On(name string, h astievent.EventHandler) {
	d.d.On(name, h)
}

func (d *Drone) Start(ctx context.Context) (err error) {
	// Make sure to execute this only once
	d.os.Do(func() {
		// Create context
		d.ctx, d.cancel = context.WithCancel(ctx)

		// Reset once
		d.oc = &sync.Once{}

		// Start dispatcher
		go d.d.Start(ctx)

		// Handle context
		go func() {
			// Wait for context to be done
			<-d.ctx.Done()

			// Signal
			d.rc.L.Lock()
			d.rc.Signal()
			d.rc.L.Unlock()
		}()

		// Handle state
		if err = d.handleState(); err != nil {
			err = errors.Wrap(err, "astitello: handling state failed")
			return
		}

		// Handle commands
		if err = d.handleCmds(); err != nil {
			err = errors.Wrap(err, "astitello: handling commands failed")
			return
		}
	})
	return
}

func (d *Drone) handleState() (err error) {
	// Create laddr
	var laddr *net.UDPAddr
	if laddr, err = net.ResolveUDPAddr("udp", ":8890"); err != nil {
		err = errors.Wrap(err, "astitello: creating laddr failed")
		return
	}

	// Listen
	if d.stateConn, err = net.ListenUDP("udp", laddr); err != nil {
		err = errors.Wrap(err, "astitello: listening failed")
		return
	}

	// Read state
	go d.readState()
	return
}

func (d *Drone) readState() {
	for {
		// Check context
		if d.ctx.Err() != nil {
			return
		}

		// Read
		b := make([]byte, 2048)
		n, err := d.stateConn.Read(b)
		if err != nil {
			if d.ctx.Err() == nil {
				astilog.Error(errors.Wrap(err, "astitello: reading state failed"))
			}
			continue
		}

		// Create state
		s, err := newState(string(b[:n]))
		if err != nil {
			astilog.Error(errors.Wrap(err, "astitello: creating state failed"))
			continue
		}

		// Dispatch
		d.d.Dispatch(StateEvent, s)
	}
}

type State struct {
	AgX    float64 // The acceleration of the "x" axis
	AgY    float64 // The acceleration of the "y" axis
	AgZ    float64 // The acceleration of the "z" axis
	Baro   float64 // The barometer measurement in cm
	Bat    int     // The percentage of the current battery level
	Height int     // The height in cm
	Mid    int     // The ID of the Mission Pad detected
	MpryX  int     // ?
	MpryY  int     // ?
	MpryZ  int     // ?
	Pitch  int     // The degree of the attitude pitch
	Roll   int     // The degree of the attitude roll
	Temph  int     // The highest temperature in degree Celsius
	Templ  int     // The lowest temperature in degree Celsius
	Time   int     // The amount of time the motor has been used
	Tof    int     // The time of flight distance in cm
	VgX    int     // The speed of the "x" axis
	VgY    int     // The speed of the "y" axis
	VgZ    int     // The speed of the "z" axis
	X      int     // The “x” coordinate detected on the Mission Pad
	Y      int     // The “y” coordinate detected on the Mission Pad
	Yaw    int     // The degree of the attitude yaw
	Z      int     // The “z” coordinate detected on the Mission Pad
}

// Seems like there's a mix up in the state format: https://tellopilots.com/threads/tello-new-firmware-01-04-78-01-released.2678/
func newState(i string) (s State, err error) {
	var n int
	if n, err = fmt.Sscanf(strings.TrimSpace(i), "mid:%d;x:%d;y:%d;z:%d;mpry:%d,%d,%d;pitch:%d;roll:%d;yaw:%d;vgx:%d;vgy:%d;vgz:%d;templ:%d;temph:%d;tof:%d;h:%d;bat:%d;baro:%f;time:%d;agx:%f;agy:%f;agz:%f;", &s.Mid, &s.X, &s.Y, &s.Z, &s.MpryX, &s.MpryY, &s.MpryZ, &s.Pitch, &s.Roll, &s.Yaw, &s.VgX, &s.VgY, &s.VgZ, &s.Templ, &s.Temph, &s.Tof, &s.Height, &s.Bat, &s.Baro, &s.Time, &s.AgX, &s.AgY, &s.AgZ); err != nil {
		err = errors.Wrap(err, "astitello: scanf failed")
		return
	} else if n != 23 {
		err = fmt.Errorf("astitello: scanf only parsed %d items, expected 10", n)
		return
	}
	return
}

func StateEventHandler(f func(s State)) astievent.EventHandler {
	return func(payload interface{}) {
		f(payload.(State))
	}
}

func (d *Drone) handleCmds() (err error) {
	// Create raddr
	var raddr *net.UDPAddr
	if raddr, err = net.ResolveUDPAddr("udp", "192.168.10.1:8889"); err != nil {
		err = errors.Wrap(err, "astitello: creating raddr failed")
		return
	}

	// Create laddr
	var laddr *net.UDPAddr
	if laddr, err = net.ResolveUDPAddr("udp", ":"); err != nil {
		err = errors.Wrap(err, "astitello: creating laddr failed")
		return
	}

	// Dial
	if d.cmdConn, err = net.DialUDP("udp", laddr, raddr); err != nil {
		err = errors.Wrap(err, "astitello: dialing failed")
		return
	}

	// Read responses
	go d.readResponses()

	// Send "command" cmd
	// In this case we don't provide a handler since once the "command" cmd has already been sent, sending a new
	// "command" cmd will result in no response at all
	if err = d.sendCmd("command", nil); err != nil {
		err = errors.Wrap(err, "astitello: sending 'command' cmd failed")
		return
	}
	return
}

func (d *Drone) readResponses() {
	for {
		// Check context
		if d.ctx.Err() != nil {
			return
		}

		// Read
		b := make([]byte, 2048)
		n, err := d.cmdConn.Read(b)
		if err != nil {
			if d.ctx.Err() == nil {
				astilog.Error(errors.Wrap(err, "astitello: reading response failed"))
			}
			continue
		}

		// Signal
		d.rc.L.Lock()
		d.lr = string(b[:n])
		d.rc.Signal()
		d.rc.L.Unlock()
	}
}

type respHandler func(resp string) error

func (d *Drone) defaultRespHandler(resp string) (err error) {
	// Check response
	if resp != "ok" {
		err = fmt.Errorf("astitello: invalid response '%s'", resp)
		return
	}
	return
}

func (d *Drone) sendCmd(cmd string, f respHandler) (err error) {
	// Lock cmd
	d.mc.Lock()
	defer d.mc.Unlock()

	// Lock resp
	d.rc.L.Lock()
	defer d.rc.L.Unlock()

	// No connection
	if d.cmdConn == nil {
		err = ErrNotStarted
		return
	}

	// Write
	if _, err = d.cmdConn.Write([]byte(cmd)); err != nil {
		err = errors.Wrap(err, "astitello: writing failed")
		return
	}

	// No handler
	if f == nil {
		return
	}

	// Wait for response
	d.rc.Wait()

	// Check context
	if d.ctx.Err() != nil {
		err = d.ctx.Err()
		return
	}

	// Custom
	if err = f(d.lr); err != nil {
		err = errors.Wrap(err, "astitello: custom handler failed")
		return
	}
	return
}

func (d *Drone) Close() {
	// Make sure to execute this only once
	d.oc.Do(func() {
		// Cancel context
		if d.cancel != nil {
			d.cancel()
		}

		// Reset once
		d.os = &sync.Once{}

		// Stop and reset dispatcher
		d.d.Stop()
		d.d.Reset()

		// Close connections
		if d.cmdConn != nil {
			d.cmdConn.Close()
		}
		if d.stateConn != nil {
			d.stateConn.Close()
		}
	})
}
