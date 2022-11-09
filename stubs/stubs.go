package stubs

import (
	"uk.ac.bris.cs/gameoflife/util"
)

var WorldLoader = "GolWorker.LoadNewWorld"
var TakeTurns = "GolWorker.ProgressToTurn"
var GetLiveCells = "GolWorker.SendLiveCells"

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

type LiveCellsCount struct {
	LiveCells int
	Turn      int
}
