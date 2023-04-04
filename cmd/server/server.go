package main

import (
	"flag"
	"gnp/server"
)

func main() {
	var c *string
	c = flag.String("c", "./config.yaml", "config")
	flag.Parse()
	server.Run(*c)
}
