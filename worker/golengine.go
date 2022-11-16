package main

import (
	"flag"
	"net"
	"net/rpc"
	"time"

	"uk.ac.bris.cs/gameoflife/stubs"
	"uk.ac.bris.cs/gameoflife/util"
)

type World struct {
	Board  [][]byte
	Height int
	Width  int
	Turn   int
}

func calculateNextState(world [][]byte, width, height int) [][]byte {

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

func getLiveCells(world [][]byte) []util.Cell {
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

func loadWorld(worldData stubs.WorldData) *World {
	world := make([][]byte, worldData.Height)
	for i := range world {
		world[i] = make([]byte, worldData.Width)
	}
	for _, cell := range worldData.LiveCells {
		world[cell.Y][cell.X] = 255
	}

	return &World{Board: world, Width: worldData.Width, Height: worldData.Height, Turn: 0}
}

type GolWorker struct{}

func (g *GolWorker) ProgressToTurn(req stubs.WorldData, res *stubs.WorldResponse) (err error) {
	client, err := rpc.Dial("tcp", req.ClientIP)
	world := loadWorld(req)
	if req.Turn <= world.Turn {
	} else {
		ticker := time.NewTicker(2 * time.Second)
		for world.Turn < req.Turn {
			select {
			case <-ticker.C:
				err = client.Call(stubs.LiveCellReport, stubs.LiveCellsCount{LiveCells: len(getLiveCells(world.Board)), Turn: world.Turn}, &stubs.Report{})
			default:
				world.Board = calculateNextState(world.Board, world.Width, world.Height)
				world.Turn++
			}
		}
	}
	res.LiveCells = getLiveCells(world.Board)
	res.Turn = world.Turn
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
