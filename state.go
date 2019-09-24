package astitello

import (
	"fmt"

	"github.com/pkg/errors"
)

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
