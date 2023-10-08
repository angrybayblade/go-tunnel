package main

import (
	"fmt"
	"os"

	"github.com/angrybayblade/tunnel/cmd"
	"github.com/urfave/cli/v2"
)

func main() {
	app := &cli.App{
		Name:  "tunnel",
		Usage: "A naive HTTP tunnel implementation",
		Commands: []*cli.Command{
			cmd.Listen,
			cmd.Forward,
			cmd.GenerateKey,
			cmd.RevokeKey,
		},
	}

	err := app.Run(os.Args)
	if err == nil || err == cmd.ErrSigterm {
		return
	}

	fmt.Println(err)
	os.Exit(1)
}
