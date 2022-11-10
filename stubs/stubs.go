package stubs

import (
	"uk.ac.bris.cs/gameoflife/util"
)

var TakeTurns = "GolWorker.ProgressToTurn"

type WorldData struct {
	LiveCells []util.Cell
	Height    int
	Width     int
	Turn      int
}

type TurnRequest struct {
	Turn int
}

type LiveCellsCount struct {
	LiveCells int
	Turn      int
}
