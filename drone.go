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
	BackEvent             = "back"
	ClockwiseEvent        = "clockwise"
	CommandEvent          = "command"
	CounterClockwiseEvent = "counter.clockwise"
	CurveEvent            = "curve"
	DownEvent             = "down"
	FlipEvent             = "flip"
	ForwardEvent          = "forward"
	GoEvent               = "go"
	LandEvent             = "land"
	LeftEvent             = "left"
	RightEvent            = "right"
	StateEvent            = "state"
	TakeOffEvent          = "take.off"
	UpEvent               = "up"
)

// Flips
const (
	FlipBack    = "b"
	FlipForward = "f"
	FlipLeft    = "l"
	FlipRight   = "r"
)

var validFlips = map[string]bool{
	FlipBack:    true,
	FlipForward: true,
	FlipLeft:    true,
	FlipRight:   true,
}

var ErrNotConnected = errors.New("astitello: not connected")

type Drone struct {
	cancel    context.CancelFunc
	cmdConn   *net.UDPConn
	ctx       context.Context
	d         *astievent.Dispatcher
	lr        string
	mc        *sync.Mutex // Locks sendCmd
	ol        *sync.Once  // Limits Close()
	oo        *sync.Once  // Limits Connect()
	rc        *sync.Cond
	stateConn *net.UDPConn
}

func New() *Drone {
	return &Drone{
		d:  astievent.NewDispatcher(),
		mc: &sync.Mutex{},
		ol: &sync.Once{},
		oo: &sync.Once{},
		rc: sync.NewCond(&sync.Mutex{}),
	}
}

func (d *Drone) On(name string, h astievent.EventHandler) {
	d.d.On(name, h)
}

func (d *Drone) Close() {
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

func (d *Drone) Connect() (err error) {
	// Make sure to execute this only once
	d.oo.Do(func() {
		// Create context
		d.ctx, d.cancel = context.WithCancel(context.Background())

		// Reset once
		d.ol = &sync.Once{}

		// Start dispatcher
		go d.d.Start(d.ctx)

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
	Pitch  int     // The degree of the attitude pitch
	Roll   int     // The degree of the attitude roll
	Temph  int     // The highest temperature in degree Celsius
	Templ  int     // The lowest temperature in degree Celsius
	Time   int     // The amount of time the motor has been used
	Tof    int     // The time of flight distance in cm
	VgX    int     // The speed of the "x" axis
	VgY    int     // The speed of the "y" axis
	VgZ    int     // The speed of the "z" axis
	Yaw    int     // The degree of the attitude yaw
}

func newState(i string) (s State, err error) {
	var n int
	if n, err = fmt.Sscanf(strings.TrimSpace(i), "pitch:%d;roll:%d;yaw:%d;vgx:%d;vgy:%d;vgz:%d;templ:%d;temph:%d;tof:%d;h:%d;bat:%d;baro:%f;time:%d;agx:%f;agy:%f;agz:%f;", &s.Pitch, &s.Roll, &s.Yaw, &s.VgX, &s.VgY, &s.VgZ, &s.Templ, &s.Temph, &s.Tof, &s.Height, &s.Bat, &s.Baro, &s.Time, &s.AgX, &s.AgY, &s.AgZ); err != nil {
		err = errors.Wrap(err, "astitello: scanf failed")
		return
	} else if n != 16 {
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
	if err = d.sendCmd("command", d.defaultRespHandler(CommandEvent, nil)); err != nil {
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
		astilog.Debugf("astitello: received resp '%s'", b[:n])

		// Signal
		d.rc.L.Lock()
		d.lr = string(b[:n])
		d.rc.Signal()
		d.rc.L.Unlock()
	}
}

type respHandler func(resp string) error

func (d *Drone) defaultRespHandler(name string, payload interface{}) respHandler {
	return func(resp string) (err error) {
		// Check response
		if resp != "ok" {
			err = errors.Wrap(errors.New(resp), "astitello: invalid response")
			return
		}

		// Publish
		d.d.Dispatch(name, payload)
		return
	}
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

// This cmd doesn't seem to be receiving any response
func (d *Drone) Emergency() (err error) {
	// Send cmd
	if err = d.sendCmd("emergency", nil); err != nil {
		err = errors.Wrap(err, "astitello: sending emergency cmd failed")
		return
	}
	return
}

func (d *Drone) TakeOff() (err error) {
	// Send cmd
	if err = d.sendCmd("takeoff", d.defaultRespHandler(TakeOffEvent, nil)); err != nil {
		err = errors.Wrap(err, "astitello: sending takeoff cmd failed")
		return
	}
	return
}

func (d *Drone) Land() (err error) {
	// Send cmd
	if err = d.sendCmd("land", d.defaultRespHandler(LandEvent, nil)); err != nil {
		err = errors.Wrap(err, "astitello: sending land cmd failed")
		return
	}
	return
}

// x is in cm
func (d *Drone) Up(x int) (err error) {
	// Send cmd
	if err = d.sendCmd(fmt.Sprintf("up %d", x), d.defaultRespHandler(UpEvent, x)); err != nil {
		err = errors.Wrap(err, "astitello: sending up cmd failed")
		return
	}
	return
}

func UpEventHandler(f func(x int)) astievent.EventHandler {
	return func(payload interface{}) {
		f(payload.(int))
	}
}

// x is in cm
func (d *Drone) Down(x int) (err error) {
	// Send cmd
	if err = d.sendCmd(fmt.Sprintf("down %d", x), d.defaultRespHandler(DownEvent, x)); err != nil {
		err = errors.Wrap(err, "astitello: sending down cmd failed")
		return
	}
	return
}

func DownEventHandler(f func(x int)) astievent.EventHandler {
	return func(payload interface{}) {
		f(payload.(int))
	}
}

// x is in cm
func (d *Drone) Left(x int) (err error) {
	// Send cmd
	if err = d.sendCmd(fmt.Sprintf("left %d", x), d.defaultRespHandler(LeftEvent, x)); err != nil {
		err = errors.Wrap(err, "astitello: sending left cmd failed")
		return
	}
	return
}

func LeftEventHandler(f func(x int)) astievent.EventHandler {
	return func(payload interface{}) {
		f(payload.(int))
	}
}

// x is in cm
func (d *Drone) Right(x int) (err error) {
	// Send cmd
	if err = d.sendCmd(fmt.Sprintf("right %d", x), d.defaultRespHandler(RightEvent, x)); err != nil {
		err = errors.Wrap(err, "astitello: sending right cmd failed")
		return
	}
	return
}

func RightEventHandler(f func(x int)) astievent.EventHandler {
	return func(payload interface{}) {
		f(payload.(int))
	}
}

// x is in cm
func (d *Drone) Forward(x int) (err error) {
	// Send cmd
	if err = d.sendCmd(fmt.Sprintf("forward %d", x), d.defaultRespHandler(ForwardEvent, x)); err != nil {
		err = errors.Wrap(err, "astitello: sending forward cmd failed")
		return
	}
	return
}

func ForwardEventHandler(f func(x int)) astievent.EventHandler {
	return func(payload interface{}) {
		f(payload.(int))
	}
}

// x is in cm
func (d *Drone) Back(x int) (err error) {
	// Send cmd
	if err = d.sendCmd(fmt.Sprintf("back %d", x), d.defaultRespHandler(BackEvent, x)); err != nil {
		err = errors.Wrap(err, "astitello: sending back cmd failed")
		return
	}
	return
}

func BackEventHandler(f func(x int)) astievent.EventHandler {
	return func(payload interface{}) {
		f(payload.(int))
	}
}

// x is in degree
func (d *Drone) Clockwise(x int) (err error) {
	// Send cmd
	if err = d.sendCmd(fmt.Sprintf("cw %d", x), d.defaultRespHandler(ClockwiseEvent, x)); err != nil {
		err = errors.Wrap(err, "astitello: sending cw cmd failed")
		return
	}
	return
}

func ClockwiseEventHandler(f func(x int)) astievent.EventHandler {
	return func(payload interface{}) {
		f(payload.(int))
	}
}

// x is in degree
func (d *Drone) CounterClockwise(x int) (err error) {
	// Send cmd
	if err = d.sendCmd(fmt.Sprintf("ccw %d", x), d.defaultRespHandler(CounterClockwiseEvent, x)); err != nil {
		err = errors.Wrap(err, "astitello: sending ccw cmd failed")
		return
	}
	return
}

func CounterClockwiseEventHandler(f func(x int)) astievent.EventHandler {
	return func(payload interface{}) {
		f(payload.(int))
	}
}

// x is one of exported Flip types
func (d *Drone) Flip(x string) (err error) {
	// Check flip
	if _, ok := validFlips[x]; !ok {
		err = fmt.Errorf("astitello: invalid flip type '%s'", x)
		return
	}

	// Send cmd
	if err = d.sendCmd(fmt.Sprintf("flip %s", x), d.defaultRespHandler(FlipEvent, x)); err != nil {
		err = errors.Wrap(err, "astitello: sending flip cmd failed")
		return
	}
	return
}

func FlipEventHandler(f func(x string)) astievent.EventHandler {
	return func(payload interface{}) {
		f(payload.(string))
	}
}

// x, y and z are in cm
// speed is in cm/s
func (d *Drone) Go(x, y, z, speed int) (err error) {
	// Send cmd
	if err = d.sendCmd(fmt.Sprintf("go %d %d %d %d", x, y, z, speed), d.defaultRespHandler(GoEvent, goEventPayload{
		speed: speed,
		x:     x,
		y:     y,
		z:     z,
	})); err != nil {
		err = errors.Wrap(err, "astitello: sending go cmd failed")
		return
	}
	return
}

type goEventPayload struct {
	speed, x, y, z int
}

func GoEventHandler(f func(x, y, z, speed int)) astievent.EventHandler {
	return func(payload interface{}) {
		p := payload.(goEventPayload)
		f(p.x, p.y, p.z, p.speed)
	}
}

// x1, x2, y1, y2, z1 and z2 are in cm
// speed is in cm/s
func (d *Drone) Curve(x1, y1, z1, x2, y2, z2, speed int) (err error) {
	// Send cmd
	if err = d.sendCmd(fmt.Sprintf("curve %d %d %d %d %d %d %d", x1, y1, z1, x2, y2, z2, speed), d.defaultRespHandler(CurveEvent, curveEventPayload{
		speed: speed,
		x1:    x1,
		x2:    x2,
		y1:    y1,
		y2:    y2,
		z1:    z1,
		z2:    z2,
	})); err != nil {
		err = errors.Wrap(err, "astitello: sending go cmd failed")
		return
	}
	return
}

type curveEventPayload struct {
	speed, x1, y1, z1, x2, y2, z2 int
}

func CurveEventHandler(f func(x1, y1, z1, x2, y2, z2, speed int)) astievent.EventHandler {
	return func(payload interface{}) {
		p := payload.(curveEventPayload)
		f(p.x1, p.y1, p.z1, p.x2, p.y2, p.z2, p.speed)
	}
}
