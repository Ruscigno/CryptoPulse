package main

import (
	"github.com/Ruscigno/cryptopulse/cmd"
	"github.com/Ruscigno/cryptopulse/logging"
	"go.uber.org/zap"
)

func main() {
	logger := logging.SetupLogger("crypto-pulse.log")
	defer logger.Sync()
	undo := zap.ReplaceGlobals(logger)
	defer undo()
	cmd.Execute()
}
