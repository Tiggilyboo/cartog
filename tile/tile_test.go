package tile

import (
	"context"
	"testing"
)

func TestTile_BadRequest(t *testing.T) {
	ctx := context.Background()
	tile, err := Tile(ctx, 1, 2, 3)
	if err != nil {
		t.Errorf("%s", err)
		return
	}

	if tile == nil {
		t.Errorf("tile empty with no error")
	}
	if tile.Tile.X != 1 || tile.Tile.Y != 2 || tile.Tile.Z != 3 {
		t.Errorf("tile returned with incorrect coordinates")
	}
	if tile.Image == nil {
		t.Errorf("tile image missing")
	}

	t.Logf("%v", tile)
}
