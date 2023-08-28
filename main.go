package main

import (
	"fmt"
	"os"

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
			Name:     "file",
			Usage:    "source file,required",
			Required: true,
		},
		&cli.StringFlag{
			Name:  "out",
			Usage: "output file,default : stdout",
		},
		&cli.StringFlag{
			Name:  "function",
			Usage: "only function default : total function in file",
		},
	}

	err := app.Run(os.Args)
	if err != nil {
		fmt.Println(err)
		return
	}
}
