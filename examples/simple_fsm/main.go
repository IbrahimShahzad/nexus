package main

import (
	"context"
	"os"

	"github.com/IbrahimShahzad/nexus"
	"github.com/rs/zerolog"
)

type MyData struct {
	ID   int
	Data string
}

func main() {

	// state: idle -> processing -> done
	logger := zerolog.New(zerolog.ConsoleWriter{Out: os.Stdout}).With().Timestamp().Logger()
	logger = logger.Level(zerolog.InfoLevel)

	machine := nexus.New[MyData]("idle",
		nexus.WithLogOutput(os.Stdout),
		nexus.WithLogConsole(), // otherwise logs are in JSON format
		nexus.WithLogLevel(zerolog.DebugLevel))

	// Register states
	if err := machine.RegisterState("processing"); err != nil {
		panic(err)
	}
	if err := machine.RegisterState("done"); err != nil {
		panic(err)
	}

	// Define Actions
	processAction := nexus.Action[MyData]{
		Name: "process_request",
		Fn: func(ctx context.Context, req *MyData) (*MyData, error) {
			req.Data = "processed: " + req.Data
			return req, nil
		},
	}

	// Add transitions
	machine.AddTransition("idle", "processing", "start", []nexus.Action[MyData]{processAction})
	machine.AddTransition("processing", "done", "complete", nil)

	ctx := context.Background()
	req := &MyData{ID: 1, Data: "test"}

	// Trigger events to move through states
	req, err := machine.Trigger(ctx, "start", req)
	if err != nil {
		panic(err)
	}

	req, err = machine.Trigger(ctx, "complete", req)
	if err != nil {
		panic(err)
	}

	logger.Info().Str("state", string(machine.GetState())).Str("data", req.Data).Msg("Reached Final State")
}
