package main

import (
	"carto/tile"
	"errors"
	"log"
)

type Coord struct {
	X float32
	Y float32
	Z float32
}

type TileCoord struct {
	X uint32
	Y uint32
	Z uint32
}

type TileGrid struct {
	location         Coord
	cache            map[TileCoord]tile.PngTile
	textures         map[TileCoord]*uint32
	tileWidth        float32
	tileHeight       float32
	screenWidth      float32
	screenHeight     float32
	ScreenTileWidth  uint32
	ScreenTileHeight uint32
}

func (c *Coord) Add(a Coord) {
	c.X += a.X
	c.Y += a.Y
	c.Z += a.Z
}

func NewTileGrid(current Coord, tileWidth uint32, tileHeight uint32, screenWidth uint32, screenHeight uint32) (*TileGrid, error) {
	if screenWidth == 0 || screenHeight == 0 {
		return nil, errors.New("screen width and height must be positive")
	}
	if tileWidth == 0 || tileHeight == 0 {
		return nil, errors.New("tile width and height must be positive")
	}

	cache := make(map[TileCoord]tile.PngTile)
	textures := make(map[TileCoord]*uint32)

	return &TileGrid{
		location:         current,
		cache:            cache,
		textures:         textures,
		tileWidth:        float32(tileWidth),
		tileHeight:       float32(tileHeight),
		screenWidth:      float32(screenWidth),
		screenHeight:     float32(screenHeight),
		ScreenTileWidth:  screenWidth / tileWidth,
		ScreenTileHeight: screenHeight / tileHeight,
	}, nil
}

func (t *TileGrid) Move(delta Coord, load chan TileCoord, expire chan TileCoord) {
	t.location.Add(delta)
	visited := make(map[TileCoord]*tile.PngTile)

	// ensure all tiles in screen space are loaded / visible
	for x := t.location.X; x < t.location.X+t.screenWidth; x += t.tileWidth {
		for y := t.location.Y; y < t.location.Y+t.screenHeight; y += t.tileHeight {
			tX := float32(x) / t.tileWidth
			tY := float32(y) / t.tileHeight

			tileCoord := TileCoord{
				X: uint32(tX),
				Y: uint32(tY),
				Z: uint32(t.location.Z),
			}

			visitedTile, exists := t.cache[tileCoord]
			if !exists {
				log.Printf("Adding tile to load %v", tileCoord)
				go func() {
					load <- tileCoord
				}()
			}

			visited[tileCoord] = &visitedTile
		}
	}

	// TODO: Better expiration logic for revisiting same areas
	/*
		go func() {
			for coord := range t.cache {
				_, exists := visited[coord]
				if !exists {
					expire <- coord
					delete(t.cache, coord)
				}
			}
		}()*/
}

func (t *TileGrid) SetTile(coord TileCoord, tile tile.PngTile, texture *uint32) {
	t.cache[coord] = tile
	if texture == nil {
		delete(t.textures, coord)
	} else {
		t.textures[coord] = texture
	}
}

func (t *TileGrid) GetLocation() *Coord {
	return &t.location
}

func (t *TileGrid) Cache() map[TileCoord]tile.PngTile {
	return t.cache
}

func (t *TileGrid) Textures() map[TileCoord]*uint32 {
	return t.textures
}
