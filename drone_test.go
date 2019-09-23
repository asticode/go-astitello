package astitello

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewState(t *testing.T) {
	s, err := newState("pitch:8;roll:9;yaw:10;vgx:11;vgy:12;vgz:13;templ:14;temph:15;tof:16;h:17;bat:18;baro:19.1;time:20;agx:21.1;agy:22.1;agz:23.1;\r\n")
	assert.Equal(t, State{AgX: 21.1, AgY: 22.1, AgZ: 23.1, Baro: 19.1, Bat: 18, Height: 17, Pitch: 8, Roll: 9, Temph: 15, Templ: 14, Time: 20, Tof: 16, VgX: 11, VgY: 12, VgZ: 13, Yaw: 10}, s)
	assert.NoError(t, err)
}
