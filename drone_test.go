package astitello

import (
	"net"
	"testing"

	"context"

	"time"

	"sync"

	"reflect"

	"github.com/pkg/errors"
	"bytes"
)

var (
	strState      = "pitch:8;roll:9;yaw:10;vgx:11;vgy:12;vgz:13;templ:14;temph:15;tof:16;h:17;bat:18;baro:19.1;time:20;agx:21.1;agy:22.1;agz:23.1;"
	expectedState = State{Acceleration: Acceleration{X: 21.1, Y: 22.1, Z: 23.1}, Attitude: Attitude{Pitch: 8, Roll: 9, Yaw: 10}, Barometer: 19.1, Battery: 18, FlightDistance: 16, FlightTime: 20, Height: 17, HighestTemperature: 15, LowestTemperature: 14, Speed: Speed{X: 11, Y: 12, Z: 13}}
)

type dialer struct {
	cancel  context.CancelFunc
	ctx     context.Context
	conn    *net.UDPConn
	h       func([]byte) []byte
	laddr   string
	raddr   string
	mt      *sync.Mutex // Locks timeout
	rs      []string
	t       *testing.T
	timeout bool
}

func newDialer(t *testing.T, laddr, raddr string) *dialer {
	return &dialer{
		laddr: laddr,
		mt:    &sync.Mutex{},
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
			d.mt.Lock()
			if d.h != nil && !d.timeout {
				if r := d.h(b[:n]); len(r) > 0 {
					if _, err := d.conn.Write(r); err != nil {
						d.mt.Unlock()
						d.t.Log(errors.Wrap(err, "test: writing failed"))
						return
					}
				}
			}
			d.mt.Unlock()
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

func setup(t *testing.T) (d *Drone, c, s, v *dialer, err error) {
	// Create cmd dialer
	c = newDialer(t, "127.0.0.1:", respAddr)

	// Set cmd handler
	c.h = func(cmd []byte) (resp []byte) {
		// Log
		t.Logf("received cmd: '%s'", cmd)

		// Switch on command
		switch string(cmd) {
		case "command", "takeoff", "land", "up 1", "down 1", "left 1", "right 1", "forward 1", "back 1", "cw 1",
			"ccw 1", "flip l", "go 1 2 3 4", "curve 1 2 3 4 5 6 7", "wifi 1 2", "speed 1", "streamon", "streamoff":
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

	// Create video dialer
	v = newDialer(t, "127.0.0.1:", videoAddr)

	// Start video dialer
	if err = v.start(); err != nil {
		err = errors.Wrap(err, "test: starting video dialer failed")
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
	d, c, s, v, err := setup(t)
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

	// Handle events
	me := &sync.Mutex{} // Locks events
	landed := false
	tookOff := false
	wg := handleEvents(t, d, &tookOff, &landed, me)

	// Test functions returning an error
	for idx, f := range []func() error{
		d.Emergency,
		d.TakeOff,
		d.Land,
		func() error { return d.Up(1) },
		func() error { return d.Down(1) },
		func() error { return d.Left(1) },
		func() error { return d.Right(1) },
		func() error { return d.Forward(1) },
		func() error { return d.Back(1) },
		func() error { return d.RotateClockwise(1) },
		func() error { return d.RotateCounterClockwise(1) },
		func() error { return d.Flip(FlipLeft) },
		func() error { return d.Go(1, 2, 3, 4) },
		func() error { return d.Curve(1, 2, 3, 4, 5, 6, 7) },
		func() error { return d.SetSticks(1, 2, 3, 4) },
		func() error { return d.SetWifi("1", "2") },
		func() error { return d.SetSpeed(1) },
		func() error { return d.StartVideo() },
		func() error { return d.StopVideo() },
	} {
		if err = f(); err != nil {
			t.Error(errors.Wrapf(err, "err %d should be nil", idx))
		}
	}

	// Wifi
	var snr int
	if snr, err = d.Wifi(); err != nil {
		t.Error(errors.Wrap(err, "err should be nil"))
	} else if snr != 100 {
		t.Errorf("expected 100, got %d", snr)
	}

	// Speed
	var speed int
	if speed, err = d.Speed(); err != nil {
		t.Error(errors.Wrap(err, "err should be nil"))
	} else if snr != 100 {
		t.Errorf("expected 100, got %d", speed)
	}

	// Cmds
	e := []string{"command", "emergency", "takeoff", "land", "up 1", "down 1", "left 1", "right 1", "forward 1",
		"back 1", "cw 1", "ccw 1", "flip l", "go 1 2 3 4", "curve 1 2 3 4 5 6 7", "rc 1 2 3 4", "wifi 1 2", "speed 1",
		"streamon", "streamoff", "wifi?", "speed?"}
	if !reflect.DeepEqual(c.rs, e) {
		t.Errorf("expected cmds %+v, got %+v", e, c.rs)
	}

	// Test events
	testEvents(t, &tookOff, &landed, wg, s, v, me)

	// Timeout
	defaultTimeout = time.Millisecond
	c.mt.Lock()
	c.timeout = true
	c.mt.Unlock()
	if err = d.command(); err == nil || errors.Cause(err) != context.DeadlineExceeded {
		t.Errorf("error should be %s", context.DeadlineExceeded)
	}
	c.mt.Lock()
	c.timeout = false
	c.mt.Unlock()
}

func handleEvents(t *testing.T, d *Drone, tookOff, landed *bool, m *sync.Mutex) (wg *sync.WaitGroup) {
	// Create wait group
	wg = &sync.WaitGroup{}

	// State events
	wg.Add(1)
	d.On(StateEvent, StateEventHandler(func(s State) {
		defer wg.Done()

		// Check state
		if s != expectedState {
			t.Errorf("expected state %+v, got %+v", expectedState, s)
		} else if d.State() != s {
			t.Error("state has not been updated")
		}
	}))

	// Video events
	wg.Add(1)
	d.On(VideoPacketEvent, VideoPacketEventHandler(func(p []byte) {
		defer wg.Done()

		// Check packet
		if !bytes.Equal(p, []byte("packet")) {
			t.Errorf("expected packet, got %s", p)
		}
	}))

	// Take off event
	wg.Add(1)
	d.On(TakeOffEvent, func(interface{}) {
		defer wg.Done()

		m.Lock()
		*tookOff = true
		m.Unlock()
	})

	// Land event
	wg.Add(1)
	d.On(LandEvent, func(interface{}) {
		defer wg.Done()

		m.Lock()
		*landed = true
		m.Unlock()
	})
	return
}

func testEvents(t *testing.T, tookOff, landed *bool, wg *sync.WaitGroup, s, v *dialer, m *sync.Mutex) {
	// Trigger state event
	if _, err := s.conn.Write([]byte(strState)); err != nil {
		t.Error(errors.Wrap(err, "test: writing state failed"))
	}

	// Trigger video event
	if _, err := v.conn.Write([]byte("packet")); err != nil {
		t.Error(errors.Wrap(err, "test: writing video packet failed"))
	}

	// Wait
	wg.Wait()

	// Lock
	m.Lock()
	defer m.Unlock()

	// Check
	if !*tookOff {
		t.Error("expected tookoff == true, got false")
	} else if !*landed {
		t.Error("expected landed == true, got false")
	}
}
