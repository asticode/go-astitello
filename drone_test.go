package astitello

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewState(t *testing.T) {
	s, err := newState("pitch:8;roll:9;yaw:10;vgx:11;vgy:12;vgz:13;templ:14;temph:15;tof:16;h:17;bat:18;baro:19.1;time:20;agx:21.1;agy:22.1;agz:23.1;\r\n")
	assert.Equal(t, State{Acceleration: Acceleration{X: 21.1, Y: 22.1, Z: 23.1}, Attitude: Attitude{Pitch: 8, Roll: 9, Yaw: 10}, Barometer: 19.1, Battery: 18, FlightDistance: 16, FlightTime: 20, Height: 17, HighestTemperature: 15, LowestTemperature: 14, Speed: Speed{X: 11, Y: 12, Z: 13}}, s)
	assert.NoError(t, err)
}
