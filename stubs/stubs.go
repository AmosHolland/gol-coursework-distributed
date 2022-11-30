package stubs

import (
	"uk.ac.bris.cs/gameoflife/util"
)

var KeyPressResponse = "StatusReceiver.KeyPressResponse"
var LiveCellReport = "StatusReceiver.LiveCellReport"

var TakeTurns = "GolBroker.MainGol"
var KeyPressed = "GolBroker.KeyPress"

var InitialiseWorker = "GolWorker.StartWorker"
var TakeTurn = "GolWorker.TakeTurn"
var WorkerKeyPress = "GolWorker.KeyPress"

type WorldData struct {
	World    [][]byte
	Height   int
	Width    int
	Turn     int
	Threads  int
	ClientIP string
}

type WorldDataBounded struct {
	Data   WorldData
	Top    int
	Bottom int
}

type BoundaryUpdate struct {
	Top    []byte
	Bottom []byte
	Turn   int
}

type WorldResponse struct {
	LiveCells []util.Cell
	Turn      int
	Liveness  byte
}

type BigWorldResponse struct {
	World [][]byte
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
