package main

import (
	"cartog/tile"
	"context"
	"errors"
	"image"
	"image/draw"
	"log"
	"runtime"
	"time"

	"github.com/go-gl/gl/v2.1/gl"
	"github.com/go-gl/glfw/v3.3/glfw"
)

const (
	TILE_X           = 256
	TILE_Y           = 256
	ZOOM_INTERVAL_MS = 300
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

func fetchTile(x uint32, y uint32, z uint32, cancel chan func()) (*tile.PngTile, error) {
	log.Printf("fetching tile (%d, %d, %d)", x, y, z)

	ctx, cancelCtx := context.WithCancel(context.Background())
	go func() {
		cancel <- cancelCtx
	}()

	t, err := tile.Tile(ctx, x, y, z)
	if err != nil {
		// Cancelled, return empty on both counts
		if ctx.Err() == context.Canceled {
			return nil, nil
		}

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

func drawTile(wState *WindowState, origin *Coord, coord *tile.TileCoord, texture *uint32) {
	ox := origin.X / float32(TILE_X)
	oy := origin.Y / float32(TILE_Y)

	// TODO: cache these
	scaleX := TILE_X / float32(wState.Width)
	scaleY := TILE_Y / float32(wState.Height)

	// Oh, the fun of the OpenGL coordinate system...
	x1 := (float32(coord.X) - ox) * scaleX
	y1 := -(float32(coord.Y) - oy) * scaleY
	x2 := x1 + scaleX
	y2 := y1 - scaleY
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
	log.Printf("Starting tile fetching goroutine")
	defer grid.Close()

	for t := range grid.TilesToLoad {
		go func(t tile.TileCoord) {
			log.Printf("tile fetch %d %d %d", t.X, t.Y, t.Z)
			pngTile, err := fetchTile(t.X, t.Y, t.Z, grid.TilesInFlight)
			if err != nil {
				log.Printf("fetch error: %s", err)
				return
			}
			// When a tile is canceled
			if pngTile == nil {
				return
			}

			// Texture already loaded
			if pngTile.Texture != nil {
				return
			}

			// Textures / GL must be done in main thread
			doWork(func() {
				log.Printf("Loading GL texture for tile %d %d", pngTile.Tile.X, pngTile.Tile.Y)
				texture, err := loadTexture(pngTile)
				if err != nil {
					return
				}
				pngTile.Texture = texture

				grid.SetTile(t, *pngTile)
			})
		}(t)
	}
}

func cleanup(grid *TileGrid) {
	log.Println("Quitting...")
	for _, tile := range grid.All() {
		if tile.Texture == nil {
			continue
		}

		gl.DeleteTextures(1, tile.Texture)
	}
}

func main() {
	if err := glfw.Init(); err != nil {
		panic(err)
	}
	defer glfw.Terminate()

	if err := gl.Init(); err != nil {
		panic(err)
	}

	windowState, err := NewWindow("Cartog")
	if err != nil {
		panic(err)
	}

	grid, err := NewTileGrid(Coord{
		X: 31 * TILE_X,
		Y: 22 * TILE_Y,
		Z: 6,
	}, TILE_X, TILE_Y, windowState.Width, windowState.Height)
	if err != nil {
		log.Fatalf("%s", err)
		return
	}

	windowState.SetResizeCallback(func(w, h uint32) {
		log.Printf("Window resized (%d, %d), resizing grid...", w, h)
		grid.Resize(w, h)
	})

	go handleTileLoading(grid)

	defer windowState.Close()

	go func() {
		for delta := range windowState.GetMovementDelta() {
			grid.Move(delta)
		}
	}()

	frames := 0
	lastTick := time.Now()

	log.Println("Starting main loop")
	for !windowState.Window.ShouldClose() {
		windowState.Window.SwapBuffers()
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
			drawTile(windowState, location, &pngTile.Tile, pngTile.Texture)
		}

		frames++
		if time.Since(lastTick) >= time.Second {
			log.Printf("FPS: %d", frames)
			lastTick = time.Now()
			frames = 0
		}
	}

	cleanup(grid)
}
