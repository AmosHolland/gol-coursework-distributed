package worker

import (
	"flag"
	"fmt"
	"net"
	"net/rpc"

	"uk.ac.bris.cs/gameoflife/stubs"
	"uk.ac.bris.cs/gameoflife/util"
)

var world [][]byte
var width int
var height int
var turn int

func calculateNextState() [][]byte {

	newWorld := make([][]byte, width)
	for i := range world {
		newWorld[i] = make([]byte, height)
	}

	for y, row := range world {
		for x, status := range row {
			score := scoreCell(x, y, width, height, world)
			var newStatus byte = 0
			if status == 255 {
				if score == 2 || score == 3 {
					newStatus = 255
				}
			} else {
				if score == 3 {
					newStatus = 255
				}
			}
			newWorld[y][x] = newStatus
		}
	}

	return newWorld
}

func scoreCell(x, y, w, h int, world [][]byte) byte {

	var score byte = 0

	for i := y - 1; i <= y+1; i++ {
		for j := x - 1; j <= x+1; j++ {
			if !(i == y && j == x) {
				score += (world[(h+i)%h][(w+j)%w] / 255)
			}
		}
	}
	return score

}

func getLiveCells() []util.Cell {
	liveCells := make([]util.Cell, 0)
	number := 0
	for y, row := range world {
		for x, status := range row {
			if status == 255 {
				liveCells = append(liveCells, util.Cell{X: x, Y: y})
				number++
			}
		}
	}
	return liveCells
}

type GolWorker struct{}

func (g *GolWorker) LoadNewWorld(req stubs.WorldData, res *stubs.Report) {
	world = make([][]byte, req.Height)
	for i := range world {
		world[i] = make([]byte, req.Width)
	}
	for _, cell := range req.LiveCells {
		world[cell.Y][cell.X] = 255
	}

	width = req.Width
	height = req.Height
	turn = 0
}

func (g *GolWorker) ProgressToTurn(req stubs.TurnRequest, res *stubs.WorldData) {
	if req.Turn <= turn {
		fmt.Println("Requested turn has already been taken")
	} else {
		for turn < req.Turn {
			world = calculateNextState()
			turn++
		}
		res.LiveCells = getLiveCells()
	}
}

func main() {
	pAddr := flag.String("port", "8030", "Port to listen on")
	flag.Parse()

	rpc.Register(&GolWorker{})
	listener, _ := net.Listen("tcp", ":"+*pAddr)
	defer listener.Close()
	rpc.Accept(listener)
}
