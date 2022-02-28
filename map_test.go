package main

import (
	"context"
	"testing"

	"github.com/paulmach/osm"
	"github.com/paulmach/osm/osmapi"
)

func TestApi_Node(t *testing.T) {
	ctx := context.Background()

	node, err := osmapi.Node(ctx, 1010)
	if err != nil {
		t.Errorf("Node not returned: %s", err)
	}
	if node.ID != 1010 {
		t.Errorf("Node ID was not 1010 != %d", node.ID)
	}
}

func TestApi_Map(t *testing.T) {
	ctx := context.Background()

	bounds := osm.Bounds{
		MinLat: 41.2528753,
		MaxLat: 41.2923814,
		MinLon: 174.6141733,
		MaxLon: 174.7787463,
	}

	ret, err := osmapi.Map(ctx, &bounds)
	if err != nil {
		t.Errorf("Node not returned: %s", err)
	}

	for _, o := range ret.Objects() {
		t.Logf("%d\n", o)
	}
}
