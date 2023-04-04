package main

import (
	"flag"
	"gnp/client"
)

func main() {
	var c *string
	c = flag.String("c", "./config.yaml", "config")
	flag.Parse()

	client.Run(*c)
}
