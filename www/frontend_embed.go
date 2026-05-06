//go:build embed_frontend

package frontend

import (
	"embed"
	"io/fs"
)

//go:embed dist
var embeddedDist embed.FS

func Dist() (fs.FS, bool) {
	dist, err := fs.Sub(embeddedDist, "dist")
	if err != nil {
		return nil, false
	}

	return dist, true
}
