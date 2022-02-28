package tile

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"image/png"
	"io/ioutil"
	"net/http"
	"time"

	"github.com/paulmach/orb/maptile"
	"github.com/paulmach/osm/osmapi"
)

var DefaultTileDatasource = &TileDatasource{
	BaseURL: "http://tile.openstreetmap.de",
	Client: &http.Client{
		Timeout: 6 * time.Minute,
	},
}

type TileDatasource struct {
	BaseURL string
	*http.Client
}

type PngTile struct {
	Tile  maptile.Tile
	Image image.Image
}

func NewPngTile(x uint32, y uint32, z uint32, pngBytes []byte) (*PngTile, error) {
	pngReader := bytes.NewReader(pngBytes)
	pngImage, err := png.Decode(pngReader)
	if err != nil {
		return nil, err
	}

	return &PngTile{
		Tile: maptile.Tile{
			X: x,
			Y: y,
			Z: maptile.Zoom(z),
		},
		Image: pngImage,
	}, nil
}

func (ds *TileDatasource) constructPngUrl(x uint32, y uint32, z uint32) string {
	url := fmt.Sprintf("%s/%d/%d/%d.png", ds.BaseURL, z, x, y)
	return url
}

func (ds *TileDatasource) getPngTileFromAPI(ctx context.Context, x uint32, y uint32, z uint32) (*PngTile, error) {
	client := ds.Client
	if client == nil {
		client = DefaultTileDatasource.Client
	}
	if client == nil {
		client = http.DefaultClient
	}

	url := ds.constructPngUrl(x, y, z)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	resp, err := client.Do(req.WithContext(ctx))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusNotFound:
		return nil, &osmapi.NotFoundError{URL: url}
	case http.StatusForbidden:
		return nil, &osmapi.ForbiddenError{URL: url}
	case http.StatusGone:
		return nil, &osmapi.GoneError{URL: url}
	case http.StatusRequestURITooLong:
		return nil, &osmapi.RequestURITooLongError{URL: url}

	case http.StatusOK:
		bodyBytes, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return nil, err
		}

		return NewPngTile(x, y, z, bodyBytes)

	default:
		return nil, &osmapi.UnexpectedStatusCodeError{URL: url}
	}
}

func (ds *TileDatasource) Tile(ctx context.Context, x uint32, y uint32, z uint32) (*PngTile, error) {
	return ds.getPngTileFromAPI(ctx, x, y, z)
}

func Tile(ctx context.Context, x uint32, y uint32, z uint32) (*PngTile, error) {
	return DefaultTileDatasource.Tile(ctx, x, y, z)
}
