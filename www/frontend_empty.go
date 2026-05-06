//go:build !embed_frontend

package frontend

import "io/fs"

func Dist() (fs.FS, bool) {
	return nil, false
}
