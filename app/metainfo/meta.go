package metainfo

import "time"

func init() {
	if BuildTime == "" {
		BuildTime = time.Now().Format(time.RFC3339)
	}
}

var Version = "dev-build"
var BuildTime = ""
var ShaVer = "undefined"
