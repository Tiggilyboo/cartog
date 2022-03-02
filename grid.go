package main

import (
	"cartog/tile"
	"errors"
	"log"
	"sync"
)

const (
	MAX_ZOOM = 17
)

type Coord struct {
	X float32
	Y float32
	Z float32
}

type TileGrid struct {
	location         Coord
	tileWidth        float32
	tileHeight       float32
	halfTileWidth    float32
	halfTileHeight   float32
	screenWidth      float32
	screenHeight     float32
	cache            map[tile.TileCoord]tile.PngTile
	loading          sync.Map
	drawable         map[tile.TileCoord]*tile.PngTile
	ScreenTileWidth  uint32
	ScreenTileHeight uint32
	TilesToLoad      chan tile.TileCoord
	TilesToExpire    chan tile.TileCoord
	CancelInflight   chan bool
}

func (c *Coord) Add(a Coord) {
	c.X += a.X
	c.Y += a.Y
	if a.Z < 0 {
		c.X /= 2.0
		c.Y /= 2.0
		c.Z += a.Z
	} else if a.Z > 0 {
		c.X *= 2.0
		c.Y *= 2.0
		c.Z += a.Z
		if c.Z > MAX_ZOOM {
			c.Z = MAX_ZOOM
		}
		return
	}

}

func NewTileGrid(current Coord, tileWidth uint32, tileHeight uint32, screenWidth uint32, screenHeight uint32) (*TileGrid, error) {
	if screenWidth == 0 || screenHeight == 0 {
		return nil, errors.New("screen width and height must be positive")
	}
	if tileWidth == 0 || tileHeight == 0 {
		return nil, errors.New("tile width and height must be positive")
	}

	return &TileGrid{
		location:       current,
		cache:          make(map[tile.TileCoord]tile.PngTile),
		loading:        sync.Map{},
		TilesToLoad:    make(chan tile.TileCoord),
		TilesToExpire:  make(chan tile.TileCoord),
		CancelInflight: make(chan bool),

		tileWidth:        float32(tileWidth),
		tileHeight:       float32(tileHeight),
		halfTileWidth:    float32(tileWidth) / 2.0,
		halfTileHeight:   float32(tileHeight) / 2.0,
		screenWidth:      float32(screenWidth),
		screenHeight:     float32(screenHeight),
		ScreenTileWidth:  screenWidth / tileWidth,
		ScreenTileHeight: screenHeight / tileHeight,
	}, nil
}

func (t *TileGrid) forEachVisibleTile(f func(tile.TileCoord)) {
	x1 := t.location.X - t.tileWidth
	x2 := t.location.X + t.halfTileWidth + t.screenWidth
	y1 := t.location.Y - t.tileHeight
	y2 := t.location.Y + t.halfTileHeight + t.screenHeight

	for x := x1; x < x2; x += t.tileWidth {
		for y := y1; y < y2; y += t.tileHeight {
			tX := x / t.tileWidth
			tY := y / t.tileHeight

			tileCoord := tile.TileCoord{
				X: uint32(tX),
				Y: uint32(tY),
				Z: uint32(t.location.Z),
			}

			f(tileCoord)
		}
	}
}

func (t *TileGrid) Move(delta Coord) {
	t.location.Add(delta)

	// Cancel any inflight requests before loading a new set of tiles
	if delta.Z != 0 {
		t.CancelInflight <- true
	}

	// ensure all tiles in screen space are loaded / visible
	t.forEachVisibleTile(func(tileCoord tile.TileCoord) {
		go func() {
			_, exists := t.loading.Load(tileCoord)
			if exists {
				return
			}
			_, exists = t.cache[tileCoord]
			if exists {
				return
			}
			log.Printf("Adding tile to load %v", tileCoord)
			t.loading.Store(tileCoord, true)
			t.TilesToLoad <- tileCoord
		}()
	})
}

func (t *TileGrid) SetTile(coord tile.TileCoord, tile tile.PngTile) {
	t.loading.Delete(coord)
	t.cache[coord] = tile
}

func (t *TileGrid) GetLocation() *Coord {
	return &t.location
}

func (t *TileGrid) Drawable() []*tile.PngTile {
	c := uint32(t.screenWidth/t.tileWidth+t.screenHeight/t.tileHeight) + 1
	tiles := make([]*tile.PngTile, 0, c)
	i := 0

	t.forEachVisibleTile(func(tileCoord tile.TileCoord) {
		tile, exists := t.cache[tileCoord]
		if exists {
			tiles = append(tiles, &tile)
			i++
		}
	})

	return tiles
}

func (t *TileGrid) All() []*tile.PngTile {
	tiles := make([]*tile.PngTile, 0, len(t.cache))
	for _, tile := range t.cache {
		tiles = append(tiles, &tile)
	}

	return tiles
}

func (t *TileGrid) Close() {
	close(t.TilesToLoad)
	close(t.TilesToExpire)
	close(t.CancelInflight)
}
