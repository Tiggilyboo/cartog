package main

import (
	"log"
	"math"
	"time"

	"github.com/go-gl/glfw/v3.3/glfw"
)

type InputState struct {
	MoveDelta            chan Coord
	mouseButtonAction    glfw.Action
	mouseButton          glfw.MouseButton
	mousePosX            float64
	mousePosY            float64
	lastPressed          time.Time
	lastPressedX         float64
	lastPressedY         float64
	clicksWithinInterval uint
	pressed              bool
}

func NewInputState(w *glfw.Window) (*InputState, error) {
	state := &InputState{
		MoveDelta:   make(chan Coord),
		lastPressed: time.Time{},
	}

	w.SetKeyCallback(state.inputKeypressCallback)
	w.SetCharCallback(state.inputCharCallback)

	w.SetMouseButtonCallback(state.inputMouseButtonCallback)
	w.SetCursorPosCallback(state.inputCursorPosCallback)
	w.SetScrollCallback(state.inputScrollCallback)

	return state, nil
}

func (state *InputState) inputCharCallback(_ *glfw.Window, ch rune) {
	delta := Coord{}
	switch ch {
	case '-':
		delta.Z = -1.0
	case '+':
		delta.Z = 1.0
	default:
		return
	}

	state.MoveDelta <- delta
}

func (state *InputState) inputScrollCallback(_ *glfw.Window, dX, dY float64) {
	if uint32(dY) == 0 {
		return
	}
	state.MoveDelta <- Coord{
		Z: float32(dY),
	}
}

func (state *InputState) inputKeypressCallback(_ *glfw.Window, key glfw.Key, _ int, action glfw.Action, mods glfw.ModifierKey) {
	if action == glfw.Release {
		return
	}
	velocity := float32(3.0)
	if mods&glfw.ModShift != 0 {
		velocity *= 10.0
	}

	delta := Coord{}
	switch key {
	case glfw.KeyLeft:
		delta.X = -velocity
	case glfw.KeyRight:
		delta.X = velocity
	case glfw.KeyUp:
		delta.Y = -velocity
	case glfw.KeyDown:
		delta.Y = velocity
	default:
		return
	}

	state.MoveDelta <- delta
}

func (state *InputState) inputMouseButtonCallback(w *glfw.Window, button glfw.MouseButton, action glfw.Action, _ glfw.ModifierKey) {
	state.mouseButtonAction = action
	state.mouseButton = button
	log.Printf("Mouse button action %v button %v lX %f lY %f", state.mouseButtonAction, state.mouseButton, state.lastPressedX-state.mousePosX, state.lastPressedY-state.mousePosY)

	// Check if we have pressed multiple times in the click interval
	if math.Abs(state.lastPressedX-state.mousePosX) < 10.0 &&
		math.Abs(state.lastPressedY-state.mousePosY) < 10.0 &&
		time.Since(state.lastPressed) <= time.Duration(ZOOM_INTERVAL_MS)*time.Millisecond {

		log.Printf("Multiclick %d, pressed %v", state.clicksWithinInterval, state.pressed)
		state.clicksWithinInterval++

		if state.clicksWithinInterval == 2 {
			state.MoveDelta <- Coord{
				Z: 1.0,
			}
		}
	} else {
		state.clicksWithinInterval = 0
	}

	if action == glfw.Release && button == glfw.MouseButtonLeft {
		state.lastPressedX = state.mousePosX
		state.lastPressedY = state.mousePosY
		state.lastPressed = time.Now()
	}
}

func (state *InputState) inputCursorPosCallback(w *glfw.Window, xpos, ypos float64) {
	if state.mouseButton != glfw.MouseButtonLeft {
		goto setMousePos
	}

	switch state.mouseButtonAction {
	case glfw.Release:
		if state.pressed {
			state.lastPressedX = xpos
			state.lastPressedY = ypos
			state.lastPressed = time.Now()
		}
		state.pressed = false

	case glfw.Press:
		// Was already pressed (Aka Held)
		if state.pressed {
			state.MoveDelta <- Coord{
				X: float32(state.mousePosX - xpos),
				Y: float32(state.mousePosY - ypos),
			}
		} else {
			// Mouse button was released, but now pressed
			state.pressed = true
		}
	}

setMousePos:
	if state.pressed {
		state.lastPressedX = xpos
		state.lastPressedY = ypos
		state.lastPressed = time.Now()
	}
	state.mousePosX = xpos
	state.mousePosY = ypos
}

func (i *InputState) Close() {
	close(i.MoveDelta)
}
