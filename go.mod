module cartog

go 1.17

require (
	github.com/paulmach/orb v0.4.0
	github.com/paulmach/osm v0.2.2
)

require (
	github.com/go-gl/gl v0.0.0-20211210172815-726fda9656d6
	github.com/go-gl/glfw/v3.3/glfw v0.0.0-20211213063430-748e38ca8aec
	github.com/gogo/protobuf v1.3.2 // indirect
)

replace cartog/tile => ./tile
