[![GoDoc](https://godoc.org/github.com/asticode/go-astitello?status.svg)](https://godoc.org/github.com/asticode/go-astitello)

This is a Golang implementation of the DJI Tello SDK.

Up-to-date SDK documentation can be downloaded [here](https://www.ryzerobotics.com/fr/tello/downloads).

Right now this library is compatible with `v1.3.0.0` of SDK ([documentation](https://terra-1-g.djicdn.com/2d4dce68897a46b19fc717f3576b7c6a/Tello%20%E7%BC%96%E7%A8%8B%E7%9B%B8%E5%85%B3/For%20Tello/Tello%20SDK%20Documentation%20EN_1.3_1122.pdf))

# Disclaimer

Tello is a registered trademark of Ryze Tech. The author of this package is in no way affiliated with Ryze, DJI, or Intel.

Use this package at your own risk. The author(s) is/are in no way responsible for any damage caused either to or by the drone when using this software.

# Run the example

IMPORTANT: the drone will make a flip to its right during this example, make sure you have enough space around the drone!

1) Switch on your drone

2) Connect to its Wifi

3) If this is the first time you're using it, you may have to activate it using the official app

4) Run the following command:

```
$ go run example/main.go
```

5) Watch your drone take off, make a flip to its right and land! Make sure to look at the terminal output too, some valuable information were printed there!

# Use it in your code

WARNING1: the code below doesn't handle errors for readibility purposes. However you SHOULD!

WARNING2: the code below doesn't list all available methods, be sure to check the [doc](https://godoc.org/github.com/asticode/go-astitello)!

```go
// For now you need to set this logger
astilog.SetDefaultLogger()

// Create the drone
d := astitello.New()

// Handle events
d.On(astitello.TakeOffEvent, func(interface{}) { astilog.Info("drone has took off!") })

// Connect to the drone
d.Connect()

// Make sure to disconnect once everything is over
defer d.Disconnect()

// Take off
d.TakeOff()

// Flip
d.Flip(astitello.FlipRight)

// Log state
astilog.Infof("state is: %+v", d.State())

// In case you're using controllers, you can use set sticks positions directly
d.SetSticks(-20, 10, -30, 40)

// Land
d.Land()
```

# Video

TODO

# Why this library?

First off, I'd like to say there are very nice DJI Tello libraries out there such as:

- https://github.com/hybridgroup/gobot/tree/master/platforms/dji/tello
- https://github.com/SMerrony/tello

Unfortunately they seem to rely on reverse-engineering the official app which is undocumented.

If you'd rather use a library that is based on an official documentation, you've come to the right place!