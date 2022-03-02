package main

import (
	"cartog/tile"
	"context"
	"errors"
	"image"
	"image/draw"
	"log"
	"runtime"

	"github.com/go-gl/gl/v2.1/gl"
	"github.com/go-gl/glfw/v3.3/glfw"
	"github.com/paulmach/osm/osmapi"
)

const (
	SCREEN_X  = 1024
	SCREEN_Y  = 768
	TILE_X    = 256
	TILE_Y    = 256
	GL_TILE_X = float32(TILE_X) / SCREEN_X
	GL_TILE_Y = float32(TILE_Y) / SCREEN_Y
)

var glWorkPipeline = make(chan func())

func init() {
	runtime.LockOSThread()
}

func doWork(f func()) {
	done := make(chan bool)
	defer close(done)

	glWorkPipeline <- func() {
		f()
		done <- true
	}
	<-done
}

func fetchTile(x uint32, y uint32, z uint32, cancel chan bool) (*tile.PngTile, error) {
	log.Printf("fetching tile (%d, %d, %d)", x, y, z)

	ctx, cancelCtx := context.WithCancel(context.Background())
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case <-cancel:
				log.Printf("Cancelling in-flight tile fetch (%d %d %d)", x, y, z)
				cancelCtx()
			}
		}
	}()

	t, err := tile.Tile(ctx, x, y, z)
	if err != nil {
		// TODO: Handle 404 in a more meaningful way when out of bounds etc.
		var nfe *osmapi.NotFoundError
		if errors.As(err, &nfe) {
			log.Fatalf("%s", nfe.Error())
			return tile.EmptyPngTile(x, y, z, TILE_X, TILE_Y)
		}
		log.Fatalf("%s", err)
		return nil, err
	}

	return t, nil
}

func loadTexture(pngTile *tile.PngTile) (*uint32, error) {
	log.Printf("loading texture (%v)", pngTile)

	rgba := image.NewRGBA(pngTile.Image.Bounds())
	if rgba.Stride != rgba.Rect.Size().X*4 {
		return nil, errors.New("unsupported image stride")
	}
	draw.Draw(rgba, rgba.Bounds(), pngTile.Image, image.Point{0, 0}, draw.Src)

	var texture uint32
	gl.Enable(gl.TEXTURE_2D)
	gl.GenTextures(1, &texture)
	gl.BindTexture(gl.TEXTURE_2D, texture)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_MIN_FILTER, gl.LINEAR)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_MAG_FILTER, gl.LINEAR)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_S, gl.CLAMP_TO_EDGE)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_T, gl.CLAMP_TO_EDGE)
	gl.TexImage2D(
		gl.TEXTURE_2D,
		0,
		gl.RGBA,
		int32(rgba.Rect.Size().X),
		int32(rgba.Rect.Size().Y),
		0,
		gl.RGBA,
		gl.UNSIGNED_BYTE,
		gl.Ptr(rgba.Pix))

	return &texture, nil
}

func drawTile(origin *Coord, coord *tile.TileCoord, texture *uint32) {
	ox := origin.X / float32(TILE_X)
	oy := origin.Y / float32(TILE_Y)

	// Oh, the fun of the OpenGL coordinate system...
	x1 := (float32(coord.X) - ox) * GL_TILE_X
	y1 := -(float32(coord.Y) - oy) * GL_TILE_Y
	x2 := x1 + GL_TILE_X
	y2 := y1 - GL_TILE_Y
	x1 = x1*2.0 - 1.0
	x2 = x2*2.0 - 1.0
	y1 = y1*2.0 + 1.0
	y2 = y2*2.0 + 1.0

	gl.BindTexture(gl.TEXTURE_2D, *texture)
	gl.Begin(gl.QUADS)

	gl.TexCoord2f(0, 0)
	gl.Vertex3f(x1, y1, 1)
	gl.TexCoord2f(1, 0)
	gl.Vertex3f(x2, y1, 1)
	gl.TexCoord2f(1, 1)
	gl.Vertex3f(x2, y2, 1)
	gl.TexCoord2f(0, 1)
	gl.Vertex3f(x1, y2, 1)

	gl.End()
}

func handleTileLoading(grid *TileGrid) {
	go func() {
		log.Printf("Starting tile fetching goroutine")
		defer grid.Close()

		for t := range grid.TilesToLoad {
			go func(t tile.TileCoord) {
				log.Printf("texture request %d %d", t.X, t.Y)
				pngTile, err := fetchTile(t.X, t.Y, t.Z, grid.CancelInflight)
				if err != nil {
					log.Fatalf("%s", err)
					return
				}

				// Textures / GL must be done in main thread
				doWork(func() {
					log.Printf("Loading GL texture for tile %d %d", pngTile.Tile.X, pngTile.Tile.Y)
					texture, err := loadTexture(pngTile)
					if err != nil {
						log.Fatalf("%s", err)
						return
					}
					pngTile.Texture = texture

					grid.SetTile(t, *pngTile)
				})
			}(t)
		}
	}()
}

func bindInput(w *glfw.Window) (delta chan Coord) {
	var mouseButtonAction glfw.Action = 0
	var mouseButton glfw.MouseButton = 0
	mousePosX := 0.0
	mousePosY := 0.0
	pressed := false

	delta = make(chan Coord)

	w.SetKeyCallback(func(_ *glfw.Window, key glfw.Key, _ int, action glfw.Action, mods glfw.ModifierKey) {
		if action == glfw.Release {
			return
		}
		velocity := float32(3.0)
		if mods&glfw.ModShift != 0 {
			velocity *= 10.0
		}

		switch key {
		case glfw.KeyLeft:
			delta <- Coord{
				X: -velocity,
			}
		case glfw.KeyRight:
			delta <- Coord{
				X: velocity,
			}
		case glfw.KeyUp:
			delta <- Coord{
				Y: -velocity,
			}
		case glfw.KeyDown:
			delta <- Coord{
				Y: velocity,
			}
		case glfw.KeyMinus:
			delta <- Coord{
				Z: -1.0,
			}
		case glfw.KeyEqual:
			if mods&glfw.ModShift != 0 {
				delta <- Coord{
					Z: 1.0,
				}
			}
		}
	})

	w.SetMouseButtonCallback(func(_ *glfw.Window, button glfw.MouseButton, action glfw.Action, _ glfw.ModifierKey) {
		mouseButtonAction = action
		mouseButton = button
	})

	w.SetCursorPosCallback(func(_ *glfw.Window, xpos float64, ypos float64) {
		if mouseButton != glfw.MouseButtonLeft {
			return
		}
		log.Printf("mouse button %v action %v (%f, %f)", mouseButton, mouseButtonAction, mousePosX, mousePosY)

		switch mouseButtonAction {
		case glfw.Release:
			if !pressed {
				return
			}
			delta <- Coord{
				X: float32(mousePosX - xpos),
				Y: float32(mousePosY - ypos),
			}
			pressed = false

		case glfw.Press:
			if pressed {
				mousePosX -= xpos
				mousePosY -= ypos

				delta <- Coord{
					X: float32(mousePosX),
					Y: float32(mousePosY),
				}
				pressed = false
				return
			}

			mousePosX = xpos
			mousePosY = ypos
			pressed = true
		}
	})

	return delta
}

func main() {
	if err := glfw.Init(); err != nil {
		panic(err)
	}
	defer glfw.Terminate()

	glfw.WindowHint(glfw.Resizable, glfw.True)
	glfw.WindowHint(glfw.ContextVersionMajor, 2)
	glfw.WindowHint(glfw.ContextVersionMinor, 1)

	window, err := glfw.CreateWindow(SCREEN_X, SCREEN_Y, "Cartog", nil, nil)
	if err != nil {
		panic(err)
	}
	window.MakeContextCurrent()

	if err := gl.Init(); err != nil {
		panic(err)
	}

	grid, err := NewTileGrid(Coord{
		X: 31 * TILE_X,
		Y: 22 * TILE_Y,
		Z: 6,
	}, TILE_X, TILE_Y, SCREEN_X, SCREEN_Y)
	if err != nil {
		log.Fatalf("%s", err)
		return
	}

	go handleTileLoading(grid)

	inputDelta := bindInput(window)
	defer close(inputDelta)

	go func() {
		for delta := range inputDelta {
			grid.Move(delta)
		}
	}()

	log.Println("Starting main loop")
	for !window.ShouldClose() {
		window.SwapBuffers()
		glfw.PollEvents()

		gl.Clear(gl.COLOR_BUFFER_BIT | gl.DEPTH_BUFFER_BIT)
		gl.LoadIdentity()

		// Check for any work in the GL pipeline
		select {
		case f := <-glWorkPipeline:
			f()
		default:
		}

		// Draw the map tiles from the cache of loaded textures
		location := grid.GetLocation()
		for _, pngTile := range grid.Drawable() {
			if pngTile == nil {
				break
			}
			if pngTile.Texture == nil {
				continue
			}
			drawTile(location, &pngTile.Tile, pngTile.Texture)
		}
	}

	log.Println("Quitting...")
	for _, tile := range grid.All() {
		if tile.Texture == nil {
			continue
		}

		gl.DeleteTextures(1, tile.Texture)
	}
}
