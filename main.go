package main

import (
	"os"

	log "github.com/liudanking/goutil/logutil"

	"github.com/urfave/cli"
)

func main() {
	app := cli.NewApp()
	app.Version = "v0.1"
	app.Usage = "gen pb from function"

	app.EnableBashCompletion = true
	app.Action = gen
	app.Flags = []cli.Flag{
		&cli.StringFlag{
			Name:  "file",
			Usage: "source file or dir, default: current dir",
		},
		&cli.StringFlag{
			Name:  "out",
			Usage: "output file",
		},
	}

	err := app.Run(os.Args)
	if err != nil {
		log.Error(err.Error())
		return
	}
}
