package main

import (
	"flag"

	"github.com/xychu/throttle/cmd/app"
	"github.com/xychu/throttle/cmd/app/options"

	"github.com/sirupsen/logrus"
	"github.com/onrik/logrus/filename"
)

func init() {
	// Add filename as one of the fields of the structured log message.
	filenameHook := filename.NewHook()
	filenameHook.Field = "filename"
	logrus.AddHook(filenameHook)
}

func main(){
	s := options.NewServerOption()
	s.AddFlags(flag.CommandLine)

	flag.Parse()

	if err := app.Run(s); err != nil {
		logrus.Fatalf("ai ops is exiting du to error:%s.\n", err)
	}
}
