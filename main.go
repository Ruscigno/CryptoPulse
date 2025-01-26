package main

import (
	"github.com/Ruscigno/stockscreener/cmd"
	"github.com/Ruscigno/stockscreener/logging"
	"go.uber.org/zap"
)

func main() {
	logger := logging.SetupLogger("crypto-pulse.log")
	defer logger.Sync()
	undo := zap.ReplaceGlobals(logger)
	defer undo()
	cmd.Execute()
}
