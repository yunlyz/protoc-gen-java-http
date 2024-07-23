package main

import (
	"flag"
	"fmt"
	"google.golang.org/protobuf/compiler/protogen"
	"os"
)

var ProtocVersion = ""

func main() {
	flag.Parse()

	options := protogen.Options{
		ParamFunc: flag.CommandLine.Set,
	}
	options.Run(func(plugin *protogen.Plugin) error {
		ProtocVersion = plugin.Request.CompilerVersion.String()

		spring := NewSpringBoot(plugin)
		return spring.Generate()
	})
}

func ErrorOutput(msg string) {
	fmt.Fprintf(os.Stderr, "\u001b[31;1mError\u001b[0m: %s\n", msg)
	os.Exit(0)
}
