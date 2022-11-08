package stubs

import (
	"uk.ac.bris.cs/gameoflife/util"
)

var WorldLoader = "GolWorker.LoadNewWorld"
var TakeTurns = "GolWorker.ProgressToTurn"

type WorldData struct {
	LiveCells []util.Cell
	Height    int
	Width     int
}

type TurnRequest struct {
	Turn int
}

type Report struct {
	Message string
}
