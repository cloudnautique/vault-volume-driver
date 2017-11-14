package main

import (
	"os"

	flexvol "github.com/rancher/rancher-flexvol"
)

var VERSION = "v0.0.0-dev"

func main() {
	backend := &FlexVol{}

	app := flexvol.NewApp(backend)
	app.Version = VERSION

	app.Run(os.Args)
}
