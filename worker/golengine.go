package main

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
var pausePlay chan bool = make(chan bool)

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

func (g *GolWorker) LoadNewWorld(req stubs.WorldData, res *stubs.Report) (err error) {
	fmt.Println("Gaming1")
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

	return
}

func (g *GolWorker) ProgressToTurn(req stubs.TurnRequest, res *stubs.WorldData) (err error) {
	fmt.Println("Gaming")
	if req.Turn <= turn {
		fmt.Println("Requested turn has already been taken")
	} else {
		for turn < req.Turn {
			select {
			case <-pausePlay:
				<-pausePlay
			default:
				world = calculateNextState()
				turn++
			}
		}
	}
	res.LiveCells = getLiveCells()
	return
}

func (g *GolWorker) SendLiveCells(req stubs.TurnRequest, res *stubs.LiveCellsCount) (err error) {
	pausePlay <- true
	fmt.Println("ahaha")
	res.LiveCells = len(getLiveCells())
	res.Turn = turn
	pausePlay <- true
	return
}

func main() {
	pAddr := flag.String("port", "8030", "Port to listen on")
	flag.Parse()

	rpc.Register(&GolWorker{})
	listener, _ := net.Listen("tcp", ":"+*pAddr)
	defer listener.Close()
	rpc.Accept(listener)
}
