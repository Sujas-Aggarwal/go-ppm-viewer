package main

import (
	"log"
	"os"

	"github.com/Sujas-Aggarwal/go-ppm-viewer/internals/ppmviewer"
)

func main() {
	if len(os.Args) != 2 {
		log.Fatal("usage: ppm-viewer <file.ppm>")
	}

	if err := ppmviewer.Run(os.Args[1]); err != nil {
		log.Fatal(err)
	}
}
