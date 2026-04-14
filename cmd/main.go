package main

import "github.com/alecthomas/kong"

type CLI struct {
	Serve ServeCmd `cmd:"" default:"withargs" help:"Start the Clock Keeper server."`
}

func main() {
	cli := CLI{}
	ctx := kong.Parse(&cli,
		kong.Name("clockkeeper"),
		kong.Description("Digital companion for Blood on the Clocktower"),
	)
	err := ctx.Run()
	ctx.FatalIfErrorf(err)
}
