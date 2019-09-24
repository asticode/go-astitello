package astitello

import (
	"net"
	"testing"

	"context"

	"time"

	"sync"

	"reflect"

	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
)

type dialer struct {
	cancel  context.CancelFunc
	ctx     context.Context
	conn    *net.UDPConn
	h       func([]byte) []byte
	laddr   string
	raddr   string
	rs      []string
	t       *testing.T
	timeout bool
}

func newDialer(t *testing.T, laddr, raddr string) *dialer {
	return &dialer{
		laddr: laddr,
		raddr: raddr,
		t:     t,
	}
}

func (d *dialer) start() (err error) {
	// Create context
	d.ctx, d.cancel = context.WithCancel(context.Background())

	// Create raddr
	var raddr *net.UDPAddr
	if raddr, err = net.ResolveUDPAddr("udp", d.raddr); err != nil {
		err = errors.Wrap(err, "test: creating raddr failed")
		return
	}

	// Create laddr
	var laddr *net.UDPAddr
	if laddr, err = net.ResolveUDPAddr("udp", d.laddr); err != nil {
		err = errors.Wrap(err, "test: creating laddr failed")
		return
	}

	// Dial
	if d.conn, err = net.DialUDP("udp", laddr, raddr); err != nil {
		err = errors.Wrap(err, "test: dialing failed")
		return
	}

	// Read
	go func() {
		for {
			// Check context
			if d.ctx.Err() != nil {
				return
			}

			// Read
			b := make([]byte, 2048)
			n, err := d.conn.Read(b)
			if err != nil {
				if d.ctx.Err() == nil {
					d.t.Log(errors.Wrap(err, "test: reading failed"))
				}
				continue
			}

			// Append
			d.rs = append(d.rs, string(b[:n]))

			// Handle
			if d.h != nil && !d.timeout {
				if r := d.h(b[:n]); len(r) > 0 {
					if _, err := d.conn.Write(r); err != nil {
						d.t.Log(errors.Wrap(err, "test: writing failed"))
						return
					}
				}
			}
		}
	}()
	return
}

func (d *dialer) close() {
	if d.cancel != nil {
		d.cancel()
	}
	if d.conn != nil {
		d.conn.Close()
	}
}

func setup(t *testing.T) (d *Drone, c *dialer, s *dialer, err error) {
	// Create cmd dialer
	c = newDialer(t, "127.0.0.1:", respAddr)

	// Set cmd handler
	c.h = func(cmd []byte) (resp []byte) {
		// Log
		t.Logf("received cmd: '%s'", cmd)

		// Switch on command
		switch string(cmd) {
		case "command", "takeoff", "land", "up 1", "down 1", "left 1", "right 1", "forward 1", "back 1", "cw 1",
			"ccw 1", "flip l", "go 1 2 3 4", "curve 1 2 3 4 5 6 7", "wifi 1 2", "speed 1":
			resp = []byte("ok")
		case "speed?":
			resp = []byte("100.0")
		case "wifi?":
			resp = []byte("100")
		}
		return
	}

	// Start cmd listener
	if err = c.start(); err != nil {
		err = errors.Wrap(err, "test: starting cmd listener failed")
		return
	}

	// Create state dialer
	s = newDialer(t, "127.0.0.1:", stateAddr)

	// Start state dialer
	if err = s.start(); err != nil {
		err = errors.Wrap(err, "test: starting state dialer failed")
		return
	}

	// Update defaults
	cmdAddr = c.conn.LocalAddr().String()

	// Create drone
	d = New()
	return
}

func TestDrone(t *testing.T) {
	// Set up
	d, c, s, err := setup(t)
	if err != nil {
		t.Error(errors.Wrap(err, "test: setting up failed"))
	}

	// Make sure to close everything properly
	defer func() {
		c.close()
		s.close()
	}()

	// Connect
	if err = d.Connect(); err != nil {
		t.Error(errors.Wrap(err, "test: connecting to drone failed"))
	}
	defer d.Disconnect()

	// State
	wg := &sync.WaitGroup{}
	wg.Add(1)
	d.On(StateEvent, StateEventHandler(func(s State) {
		defer wg.Done()

		// Check state
		if s != expectedState {
			t.Errorf("expected state %+v, got %+v", expectedState, s)
		}

		// Check state has been updated
		if d.State() != s {
			t.Error("state has not been updated")
		}
	}))
	if _, err = s.conn.Write([]byte(strState)); err != nil {
		t.Error(errors.Wrap(err, "test: writing state failed"))
	}
	wg.Wait()

	// Emergency
	if err = d.Emergency(); err != nil {
		t.Error(errors.Wrap(err, "err should be nil"))
	}

	// Take off
	tookOff := false
	d.On(TakeOffEvent, func(interface{}) { tookOff = true })
	if err = d.TakeOff(); err != nil {
		t.Error(errors.Wrap(err, "err should be nil"))
	}
	time.Sleep(time.Millisecond)
	if !tookOff {
		t.Error("expected tookoff == true, got false")
	}

	// Land
	landed := false
	d.On(LandEvent, func(interface{}) { landed = true })
	if err = d.Land(); err != nil {
		t.Error(errors.Wrap(err, "err should be nil"))
	}
	time.Sleep(time.Millisecond)
	if !landed {
		t.Error("expected landed == true, got false")
	}

	// Up
	if err = d.Up(1); err != nil {
		t.Error(errors.Wrap(err, "err should be nil"))
	}

	// Down
	if err = d.Down(1); err != nil {
		t.Error(errors.Wrap(err, "err should be nil"))
	}

	// Left
	if err = d.Left(1); err != nil {
		t.Error(errors.Wrap(err, "err should be nil"))
	}

	// Right
	if err = d.Right(1); err != nil {
		t.Error(errors.Wrap(err, "err should be nil"))
	}

	// Forward
	if err = d.Forward(1); err != nil {
		t.Error(errors.Wrap(err, "err should be nil"))
	}

	// Back
	if err = d.Back(1); err != nil {
		t.Error(errors.Wrap(err, "err should be nil"))
	}

	// Rotate cw
	if err = d.RotateClockwise(1); err != nil {
		t.Error(errors.Wrap(err, "err should be nil"))
	}

	// Rotate ccw
	if err = d.RotateCounterClockwise(1); err != nil {
		t.Error(errors.Wrap(err, "err should be nil"))
	}

	// Flip
	if err = d.Flip(FlipLeft); err != nil {
		t.Error(errors.Wrap(err, "err should be nil"))
	}

	// Go
	if err = d.Go(1, 2, 3, 4); err != nil {
		t.Error(errors.Wrap(err, "err should be nil"))
	}

	// Curve
	if err = d.Curve(1, 2, 3, 4, 5, 6, 7); err != nil {
		t.Error(errors.Wrap(err, "err should be nil"))
	}

	// Sticks
	if err = d.SetSticks(1, 2, 3, 4); err != nil {
		t.Error(errors.Wrap(err, "err should be nil"))
	}

	// Wifi
	if err = d.SetWifi("1", "2"); err != nil {
		t.Error(errors.Wrap(err, "err should be nil"))
	}
	var snr int
	if snr, err = d.Wifi(); err != nil {
		t.Error(errors.Wrap(err, "err should be nil"))
	} else if snr != 100 {
		t.Errorf("expected 100, got %d", snr)
	}

	// Speed
	if err = d.SetSpeed(1); err != nil {
		t.Error(errors.Wrap(err, "err should be nil"))
	}
	var speed int
	if speed, err = d.Speed(); err != nil {
		t.Error(errors.Wrap(err, "err should be nil"))
	} else if snr != 100 {
		t.Errorf("expected 100, got %d", speed)
	}

	// Cmds
	e := []string{"command", "emergency", "takeoff", "land", "up 1", "down 1", "left 1", "right 1", "forward 1",
		"back 1", "cw 1", "ccw 1", "flip l", "go 1 2 3 4", "curve 1 2 3 4 5 6 7", "rc 1 2 3 4", "wifi 1 2", "wifi?",
		"speed 1", "speed?"}
	if !reflect.DeepEqual(c.rs, e) {
		t.Errorf("expected cmds %+v, got %+v", e, c.rs)
	}

	// Timeout
	defaultTimeout = time.Millisecond
	c.timeout = true
	if err = d.command(); err == nil || errors.Cause(err) != context.DeadlineExceeded {
		t.Errorf("error should be %s", context.DeadlineExceeded)
	}
	c.timeout = false
}

var (
	strState      = "pitch:8;roll:9;yaw:10;vgx:11;vgy:12;vgz:13;templ:14;temph:15;tof:16;h:17;bat:18;baro:19.1;time:20;agx:21.1;agy:22.1;agz:23.1;"
	expectedState = State{Acceleration: Acceleration{X: 21.1, Y: 22.1, Z: 23.1}, Attitude: Attitude{Pitch: 8, Roll: 9, Yaw: 10}, Barometer: 19.1, Battery: 18, FlightDistance: 16, FlightTime: 20, Height: 17, HighestTemperature: 15, LowestTemperature: 14, Speed: Speed{X: 11, Y: 12, Z: 13}}
)

func TestNewState(t *testing.T) {
	s, err := newState(strState)
	assert.Equal(t, expectedState, s)
	assert.NoError(t, err)
}
