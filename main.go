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
		&cli.StringFlag{
			Name:  "struct",
			Usage: "only suruct default : total function in file",
		},
		&cli.BoolFlag{
			Name:  "rpc",
			Usage: "gen rpc",
		},
		&cli.BoolFlag{
			Name:  "vv",
			Usage: "stdout struct2pb for rpc",
		},
		&cli.BoolFlag{
			Name:  "vvv",
			Usage: "stdout struct2pb for dao",
		},
	}

	err := app.Run(os.Args)
	if err != nil {
		fmt.Println(err)
		return
	}
}
