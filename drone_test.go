package astitello

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewState(t *testing.T) {
	s, err := newState("mid:1;x:2;y:3;z:4;mpry:5,6,7;pitch:8;roll:9;yaw:10;vgx:11;vgy:12;vgz:13;templ:14;temph:15;tof:16;h:17;bat:18;baro:19.1;time:20;agx:21.1;agy:22.1;agz:23.1;\r\n")
	assert.Equal(t, State{AgX: 21.1, AgY: 22.1, AgZ: 23.1, Baro: 19.1, Bat: 18, Height: 17, Mid: 1, MpryX: 5, MpryY: 6, MpryZ: 7, Pitch: 8, Roll: 9, Temph: 15, Templ: 14, Time: 20, Tof: 16, VgX: 11, VgY: 12, VgZ: 13, X: 2, Y: 3, Yaw: 10, Z: 4}, s)
	assert.NoError(t, err)
}
