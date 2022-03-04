package main

import (
	"github.com/go-gl/glfw/v3.3/glfw"
)

type WindowState struct {
	Window         *glfw.Window
	input          *InputState
	Width          uint32
	Height         uint32
	resizeCallback func(width, height uint32)
}

func getInitialResolution() (int, int) {
	monitor := glfw.GetPrimaryMonitor()
	modes := monitor.GetVideoModes()
	firstMode := modes[0]

	return firstMode.Width, firstMode.Height
}

func NewWindow(title string) (*WindowState, error) {
	glfw.WindowHint(glfw.Resizable, glfw.True)
	glfw.WindowHint(glfw.ContextVersionMajor, 2)
	glfw.WindowHint(glfw.ContextVersionMinor, 1)

	screenW, screenH := getInitialResolution()

	window, err := glfw.CreateWindow(screenW, screenH, title, nil, nil)
	if err != nil {
		panic(err)
	}
	window.MakeContextCurrent()

	inputState, err := NewInputState(window)
	if err != nil {
		panic(err)
	}

	state := &WindowState{
		Window: window,
		Width:  uint32(screenW),
		Height: uint32(screenH),
		input:  inputState,
	}

	window.SetSizeCallback(state.windowResizeCallback)

	return state, nil
}

func (state *WindowState) Close() {
	state.input.Close()
}

func (state *WindowState) GetMovementDelta() chan Coord {
	return state.input.MoveDelta
}

func (state *WindowState) SetResizeCallback(handler func(width, height uint32)) {
	state.resizeCallback = handler
}

func (state *WindowState) windowResizeCallback(_ *glfw.Window, width, height int) {
	state.Width = uint32(width)
	state.Height = uint32(height)

	if state.resizeCallback != nil {
		go state.resizeCallback(state.Width, state.Height)
	}
}
