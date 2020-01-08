[![GoReportCard](http://goreportcard.com/badge/github.com/asticode/go-astitello)](http://goreportcard.com/report/github.com/asticode/go-astitello)
[![GoDoc](https://godoc.org/github.com/asticode/go-astitello?status.svg)](https://godoc.org/github.com/asticode/go-astitello)
[![Travis](https://travis-ci.org/asticode/go-astitello.svg?branch=master)](https://travis-ci.org/asticode/go-astitello#)
[![Coveralls](https://coveralls.io/repos/github/asticode/go-astitello/badge.svg?branch=master)](https://coveralls.io/github/asticode/go-astitello)

This is a Golang implementation of the DJI Tello SDK.

Up-to-date SDK documentation can be downloaded [here](https://www.ryzerobotics.com/fr/tello/downloads).

Right now this library is compatible with `v1.3.0.0` of SDK ([documentation](https://terra-1-g.djicdn.com/2d4dce68897a46b19fc717f3576b7c6a/Tello%20%E7%BC%96%E7%A8%8B%E7%9B%B8%E5%85%B3/For%20Tello/Tello%20SDK%20Documentation%20EN_1.3_1122.pdf))

# Disclaimer

Tello is a registered trademark of Ryze Tech. The author of this package is in no way affiliated with Ryze, DJI, or Intel.

Use this package at your own risk. The author(s) is/are in no way responsible for any damage caused either to or by the drone when using this software.

# Install the project

Run the following command:

```
$ go get -u github.com/asticode/go-astitello/...
```

# Run the example

IMPORTANT: the drone will make a flip to its right during this example, make sure you have enough space around the drone!

1) Switch on your drone

2) Connect to its Wifi

3) If this is the first time you're using it, you may have to activate it using the official app

4) If you want to test the video, install `ffmpeg` on your machine

5) Run the following command:

```
$ go run example/main.go
```

5) Watch your drone take off, make a flip to its right and land! Make sure to look at the terminal output too, some valuable information were printed there!

6) If you've installed `ffmpeg` you should also see a new file called `example.ts`. Check it out!

# Use it in your code

WARNING1: the code below doesn't handle errors for readibility purposes. However you SHOULD!

WARNING2: the code below doesn't list all available methods, be sure to check out the [doc](https://godoc.org/github.com/asticode/go-astitello)!

## Set up the drone

```go
// Create logger
l := log.New(os.StdErr, "", 0)

// Create the drone
d := astitello.New(l)

// Start the drone
d.Start()

// Make sure to close the drone once everything is over
defer d.Close()
```

## Basic commands

```go
// Handle take off event
d.On(astitello.TakeOffEvent, func(interface{}) { l.Println("drone has took off!") })

// Take off
d.TakeOff()

// Flip
d.Flip(astitello.FlipRight)

// Log state
l.Printf("state is: %+v\n", d.State())

// In case you're using controllers, you can use set sticks positions directly
d.SetSticks(-20, 10, -30, 40)

// Land
d.Land()
```

## Video

```go
// Handle new video packet
d.On(astitello.VideoPacketEvent, astitello.VideoPacketEventHandler(func(p []byte) {
    l.Printf("video packet length: %d\n", len(p))
}))

// Start video
d.StartVideo()

// Make sure to stop video
defer d.StopVideo()
```

# Why this library?

First off, I'd like to say there are very nice DJI Tello libraries out there such as:

- https://github.com/hybridgroup/gobot/tree/master/platforms/dji/tello
- https://github.com/SMerrony/tello

Unfortunately they seem to rely on reverse-engineering the official app which is undocumented.

If you'd rather use a library that is based on an official documentation, you've come to the right place!

# Known problems with the SDK

- sometimes a cmd doesn't get any response back from the SDK. In that case the cmd will idle until its timeout is reached.