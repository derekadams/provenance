package main

import (
	"log"
	"os"

	"github.com/cosmos/cosmos-sdk/server"

	"github.com/provenance-io/provenance/cmd/provenanced/cmd"

	"gopkg.in/DataDog/dd-trace-go.v1/profiler"
)

func main() {
	err := profiler.Start(
		profiler.WithService("provenanced"),
		profiler.WithEnv("development"),
		profiler.WithVersion("1.7.x"),
		profiler.WithTags("test:123"),
		profiler.WithProfileTypes(
			profiler.CPUProfile,
			profiler.HeapProfile,
		),
	)
	if err != nil {
		log.Fatal(err)
	}
	defer profiler.Stop()

	rootCmd, _ := cmd.NewRootCmd()
	if err := cmd.Execute(rootCmd); err != nil {
		switch e := err.(type) {
		case server.ErrorCode:
			os.Exit(e.Code)
		default:
			os.Exit(1)
		}
	}
}
