package stubs

import (
	"uk.ac.bris.cs/gameoflife/util"
)

var TakeTurns = "GolWorker.ProgressToTurn"
var LiveCellReport = "StatusReceiver.LiveCellReport"
var KeyPressed = "GolWorker.KeyPressed"

type WorldData struct {
	LiveCells []util.Cell
	Height    int
	Width     int
	Turn      int
	ClientIP  string
}

type WorldResponse struct {
	LiveCells []util.Cell
	Turn      int
}

type TurnRequest struct {
	Turn int
}

type LiveCellsCount struct {
	LiveCells int
	Turn      int
}

type Report struct {
	Message string
}

type KeyPress struct {
	Key rune
}
