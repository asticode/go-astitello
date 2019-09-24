package astitello

import (
	"bytes"
	"context"
	"fmt"
	"net"
	"strconv"
	"sync"

	"time"

	"github.com/asticode/go-astilog"
	astievent "github.com/asticode/go-astitools/event"
	"github.com/pkg/errors"
)

const defaultTimeout = 5 * time.Second

// Events
const (
	LandEvent    = "land"
	StateEvent   = "state"
	TakeOffEvent = "take.off"
)

// Flip directions
const (
	FlipBack    = "b"
	FlipForward = "f"
	FlipLeft    = "l"
	FlipRight   = "r"
)

// ErrNotConnected is the error thrown when trying to send a cmd while not connected to the drone
var ErrNotConnected = errors.New("astitello: not connected")

// Drone represents an object capable of interacting with the SDK
type Drone struct {
	cancel    context.CancelFunc
	cmdConn   *net.UDPConn
	ctx       context.Context
	d         *astievent.Dispatcher
	lr        string
	mc        *sync.Mutex // Locks sendCmd
	ms        *sync.Mutex // Locks s
	ol        *sync.Once  // Limits Close()
	oo        *sync.Once  // Limits Connect()
	rc        *sync.Cond
	s         *State
	stateConn *net.UDPConn
}

// New creates a new Drone
func New() *Drone {
	return &Drone{
		d:  astievent.NewDispatcher(),
		mc: &sync.Mutex{},
		ms: &sync.Mutex{},
		ol: &sync.Once{},
		oo: &sync.Once{},
		rc: sync.NewCond(&sync.Mutex{}),
		s:  &State{},
	}
}

// State returns the drone's state
func (d *Drone) State() State {
	d.ms.Lock()
	defer d.ms.Unlock()
	return *d.s
}

// On adds an event handler
func (d *Drone) On(name string, h astievent.EventHandler) {
	d.d.On(name, h)
}

// Disconnect disconnects from the drone
func (d *Drone) Disconnect() {
	// Make sure to execute this only once
	d.ol.Do(func() {
		// Cancel context
		if d.cancel != nil {
			d.cancel()
		}

		// Reset once
		d.oo = &sync.Once{}

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

// Connect connects to the drone
func (d *Drone) Connect() (err error) {
	// Make sure to execute this only once
	d.oo.Do(func() {
		// Create context
		d.ctx, d.cancel = context.WithCancel(context.Background())

		// Reset once
		d.ol = &sync.Once{}

		// Start dispatcher
		go d.d.Start(d.ctx)

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
		s, err := newState(string(bytes.TrimSpace(b[:n])))
		if err != nil {
			astilog.Error(errors.Wrap(err, "astitello: creating state failed"))
			continue
		}

		// Update state
		d.ms.Lock()
		*d.s = s
		d.ms.Unlock()

		// Dispatch
		d.d.Dispatch(StateEvent, s)
	}
}

// State represents the drone's state
type State struct {
	Acceleration       Acceleration // The acceleration
	Attitude           Attitude     // The attitude
	Barometer          float64      // The barometer measurement in cm
	Battery            int          // The percentage of the current battery level
	FlightDistance     int          // The time of flight distance in cm
	FlightTime         int          // The amount of time the motor has been used
	Height             int          // The height in cm
	HighestTemperature int          // The highest temperature in degree Celsius
	LowestTemperature  int          // The lowest temperature in degree Celsius
	Speed              Speed        // The speed
}

// Acceleration represents the drone's acceleration
type Acceleration struct {
	X float64
	Y float64
	Z float64
}

// Attitude represents the drone's attitude
type Attitude struct {
	Pitch int // The degree of the attitude pitch
	Roll  int // The degree of the attitude roll
	Yaw   int // The degree of the attitude yaw
}

// Speed represents the drone's speed
type Speed struct {
	X int
	Y int
	Z int
}

func newState(i string) (s State, err error) {
	var n int
	if n, err = fmt.Sscanf(i, "pitch:%d;roll:%d;yaw:%d;vgx:%d;vgy:%d;vgz:%d;templ:%d;temph:%d;tof:%d;h:%d;bat:%d;baro:%f;time:%d;agx:%f;agy:%f;agz:%f;", &s.Attitude.Pitch, &s.Attitude.Roll, &s.Attitude.Yaw, &s.Speed.X, &s.Speed.Y, &s.Speed.Z, &s.LowestTemperature, &s.HighestTemperature, &s.FlightDistance, &s.Height, &s.Battery, &s.Barometer, &s.FlightTime, &s.Acceleration.X, &s.Acceleration.Y, &s.Acceleration.Z); err != nil {
		err = errors.Wrap(err, "astitello: scanf failed")
		return
	} else if n != 16 {
		err = fmt.Errorf("astitello: scanf only parsed %d items, expected 10", n)
		return
	}
	return
}

// StateEventHandler returns the proper EventHandler for the State event
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
	if laddr, err = net.ResolveUDPAddr("udp", ":8889"); err != nil {
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
	if err = d.sendCmd("command", defaultTimeout, defaultRespHandler); err != nil {
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

		// Log
		r := bytes.TrimSpace(b[:n])
		astilog.Debugf("astitello: received resp '%s'", r)

		// Signal
		d.rc.L.Lock()
		d.lr = string(r)
		d.rc.Signal()
		d.rc.L.Unlock()
	}
}

type respHandler func(resp string) error

func defaultRespHandler(resp string) (err error) {
	// Check response
	if resp != "ok" {
		err = errors.Wrap(errors.New(resp), "astitello: invalid response")
		return
	}
	return
}

func (d *Drone) respHandlerWithEvent(name string) respHandler {
	return func(resp string) (err error) {
		// Default
		if err = defaultRespHandler(resp); err != nil {
			return
		}

		// Dispatch
		d.d.Dispatch(name, nil)
		return
	}
}

func (d *Drone) sendCmd(cmd string, timeout time.Duration, f respHandler) (err error) {
	// Lock cmd
	d.mc.Lock()
	defer d.mc.Unlock()

	// Lock resp
	d.rc.L.Lock()
	defer d.rc.L.Unlock()

	// No connection
	if d.cmdConn == nil {
		err = ErrNotConnected
		return
	}

	// Log
	astilog.Debugf("astitello: sending cmd '%s'", cmd)

	// Write
	if _, err = d.cmdConn.Write([]byte(cmd)); err != nil {
		err = errors.Wrap(err, "astitello: writing failed")
		return
	}

	// No handler
	if f == nil {
		return
	}

	// Create context
	ctx, cancel := context.WithCancel(d.ctx)
	if timeout > 0 {
		ctx, cancel = context.WithTimeout(d.ctx, timeout)
	}
	defer cancel()

	// Handle context
	go func() {
		// Wait for context to be done
		<-ctx.Done()

		// Check error
		if d.ctx.Err() != context.Canceled && ctx.Err() != context.DeadlineExceeded {
			return
		}

		// Signal
		d.rc.L.Lock()
		d.rc.Signal()
		d.rc.L.Unlock()
	}()

	// Wait for response
	d.rc.Wait()

	// Check context
	if ctx.Err() != nil {
		err = ctx.Err()
		return
	}

	// Custom
	if err = f(d.lr); err != nil {
		err = errors.Wrap(err, "astitello: custom handler failed")
		return
	}
	return
}

// Emergency makes Tello stop all motors immediately
// This cmd doesn't seem to be receiving any response
func (d *Drone) Emergency() (err error) {
	// Send cmd
	if err = d.sendCmd("emergency", 0, nil); err != nil {
		err = errors.Wrap(err, "astitello: sending emergency cmd failed")
		return
	}
	return
}

// TakeOff makes Tello auto takeoff
func (d *Drone) TakeOff() (err error) {
	// Send cmd
	if err = d.sendCmd("takeoff", 0, d.respHandlerWithEvent(TakeOffEvent)); err != nil {
		err = errors.Wrap(err, "astitello: sending takeoff cmd failed")
		return
	}
	return
}

// Land makes Tello auto land
func (d *Drone) Land() (err error) {
	// Send cmd
	if err = d.sendCmd("land", 0, d.respHandlerWithEvent(LandEvent)); err != nil {
		err = errors.Wrap(err, "astitello: sending land cmd failed")
		return
	}
	return
}

// Up makes Tello fly up with distance x cm
func (d *Drone) Up(x int) (err error) {
	// Send cmd
	if err = d.sendCmd(fmt.Sprintf("up %d", x), 0, defaultRespHandler); err != nil {
		err = errors.Wrap(err, "astitello: sending up cmd failed")
		return
	}
	return
}

// Down makes Tello fly down with distance x cm
func (d *Drone) Down(x int) (err error) {
	// Send cmd
	if err = d.sendCmd(fmt.Sprintf("down %d", x), 0, defaultRespHandler); err != nil {
		err = errors.Wrap(err, "astitello: sending down cmd failed")
		return
	}
	return
}

// Left makes Tello fly left with distance x cm
func (d *Drone) Left(x int) (err error) {
	// Send cmd
	if err = d.sendCmd(fmt.Sprintf("left %d", x), 0, defaultRespHandler); err != nil {
		err = errors.Wrap(err, "astitello: sending left cmd failed")
		return
	}
	return
}

// Right makes Tello fly right with distance x cm
func (d *Drone) Right(x int) (err error) {
	// Send cmd
	if err = d.sendCmd(fmt.Sprintf("right %d", x), 0, defaultRespHandler); err != nil {
		err = errors.Wrap(err, "astitello: sending right cmd failed")
		return
	}
	return
}

// Forward makes Tello fly forward with distance x cm
func (d *Drone) Forward(x int) (err error) {
	// Send cmd
	if err = d.sendCmd(fmt.Sprintf("forward %d", x), 0, defaultRespHandler); err != nil {
		err = errors.Wrap(err, "astitello: sending forward cmd failed")
		return
	}
	return
}

// Back makes Tello fly back with distance x cm
func (d *Drone) Back(x int) (err error) {
	// Send cmd
	if err = d.sendCmd(fmt.Sprintf("back %d", x), 0, defaultRespHandler); err != nil {
		err = errors.Wrap(err, "astitello: sending back cmd failed")
		return
	}
	return
}

// RotateClockwise makes Tello rotate x degree clockwise
func (d *Drone) RotateClockwise(x int) (err error) {
	// Send cmd
	if err = d.sendCmd(fmt.Sprintf("cw %d", x), 0, defaultRespHandler); err != nil {
		err = errors.Wrap(err, "astitello: sending cw cmd failed")
		return
	}
	return
}

// RotateCounterClockwise makes Tello rotate x degree counter-clockwise
func (d *Drone) RotateCounterClockwise(x int) (err error) {
	// Send cmd
	if err = d.sendCmd(fmt.Sprintf("ccw %d", x), 0, defaultRespHandler); err != nil {
		err = errors.Wrap(err, "astitello: sending ccw cmd failed")
		return
	}
	return
}

// Flip makes Tello flip in the specified direction
// Check out Flip... constants for available flip directions
func (d *Drone) Flip(x string) (err error) {
	// Send cmd
	if err = d.sendCmd(fmt.Sprintf("flip %s", x), 0, defaultRespHandler); err != nil {
		err = errors.Wrap(err, "astitello: sending flip cmd failed")
		return
	}
	return
}

// Go makes Tello fly to x y z in speed (cm/s)
func (d *Drone) Go(x, y, z, speed int) (err error) {
	// Send cmd
	if err = d.sendCmd(fmt.Sprintf("go %d %d %d %d", x, y, z, speed), 0, defaultRespHandler); err != nil {
		err = errors.Wrap(err, "astitello: sending go cmd failed")
		return
	}
	return
}

// Curve makes Tello fly a curve defined by the current and two given coordinates with speed (cm/s)
func (d *Drone) Curve(x1, y1, z1, x2, y2, z2, speed int) (err error) {
	// Send cmd
	if err = d.sendCmd(fmt.Sprintf("curve %d %d %d %d %d %d %d", x1, y1, z1, x2, y2, z2, speed), 0, defaultRespHandler); err != nil {
		err = errors.Wrap(err, "astitello: sending go cmd failed")
		return
	}
	return
}

// SetSticks sends RC control via four channels
// All values are between -100 and 100
// lr: left/right
// fb: forward/backward
// ud: up/down
// y: yawn
// This cmd doesn't seem to be receiving any response
func (d *Drone) SetSticks(lr, fb, ud, y int) (err error) {
	// Send cmd
	if err = d.sendCmd(fmt.Sprintf("rc %d %d %d %d", lr, fb, ud, y), defaultTimeout, nil); err != nil {
		err = errors.Wrap(err, "astitello: sending rc cmd failed")
		return
	}
	return
}

// SetWifi sets Wi-Fi with SSID password
// I couldn't make this work (it returned 'error' even though the SSID was changed but the password was not)
// If anyone manages to make it work, create an issue in github, I'm really interested in how you managed that :D
func (d *Drone) SetWifi(ssid, password string) (err error) {
	// Send cmd
	if err = d.sendCmd(fmt.Sprintf("wifi %s %s", ssid, password), defaultTimeout, defaultRespHandler); err != nil {
		err = errors.Wrap(err, "astitello: sending wifi cmd failed")
		return
	}
	return
}

// Wifi returns the Wifi SNR
func (d *Drone) Wifi() (snr string, err error) {
	// Send cmd
	// It returns "100.0"
	if err = d.sendCmd("wifi?", defaultTimeout, func(resp string) (err error) {
		// Set snr
		snr = resp
		return
	}); err != nil {
		err = errors.Wrap(err, "astitello: sending wifi? cmd failed")
		return
	}
	return
}

// SetSpeed sets speed to x cm/s
func (d *Drone) SetSpeed(x int) (err error) {
	// Send cmd
	if err = d.sendCmd(fmt.Sprintf("speed %d", x), defaultTimeout, defaultRespHandler); err != nil {
		err = errors.Wrap(err, "astitello: sending speed cmd failed")
		return
	}
	return
}

// Speed returns the current speed (cm/s)
func (d *Drone) Speed() (x int, err error) {
	// Send cmd
	// It returns "100.0"
	if err = d.sendCmd("speed?", defaultTimeout, func(resp string) (err error) {
		// Parse
		var f float64
		if f, err = strconv.ParseFloat(resp, 64); err != nil {
			err = errors.Wrapf(err, "astitello: parsing float %s failed", resp)
			return
		}

		// Set speed
		x = int(f)
		return
	}); err != nil {
		err = errors.Wrap(err, "astitello: sending speed? cmd failed")
		return
	}
	return
}
