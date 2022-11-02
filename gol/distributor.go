package gol

import (
	"fmt"

	"uk.ac.bris.cs/gameoflife/util"
)

type distributorChannels struct {
	events     chan<- Event
	ioCommand  chan<- ioCommand
	ioIdle     <-chan bool
	ioFilename chan<- string
	ioOutput   chan<- uint8
	ioInput    <-chan uint8
}

// distributor divides the work between workers and interacts with other goroutines.

func calculateNextState(p Params, world [][]byte) [][]byte {

	newWorld := make([][]byte, p.ImageHeight)
	for i := range world {
		newWorld[i] = make([]byte, p.ImageWidth)
	}

	for y, row := range world {
		for x, status := range row {
			score := scoreCell(x, y, p.ImageWidth, p.ImageHeight, world)
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

func getLiveCells(world [][]byte, p Params) []util.Cell {
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

func distributor(p Params, c distributorChannels) {

	// TODO: Create a 2D slice to store the world.
	world := make([][]byte, p.ImageHeight)
	for i := range world {
		world[i] = make([]byte, p.ImageWidth)
	}

	c.ioCommand <- ioInput
	fileName := fmt.Sprint(p.ImageWidth, "x", p.ImageHeight)
	fmt.Println(fileName)
	c.ioFilename <- fileName

	for y, row := range world {
		for x, _ := range row {
			world[y][x] = <-c.ioInput
		}
	}

	turn := 0

	// TODO: Execute all turns of the Game of Life.

	for turn < p.Turns {
		world = calculateNextState(p, world)
		turn++
	}

	// TODO: Report the final state using FinalTurnCompleteEvent.

	c.events <- FinalTurnComplete{CompletedTurns: turn, Alive: getLiveCells(world, p)}
	// Make sure that the Io has finished any output before exiting.
	c.ioCommand <- ioCheckIdle
	<-c.ioIdle

	c.events <- StateChange{turn, Quitting}

	// Close the channel to stop the SDL goroutine gracefully. Removing may cause deadlock.
	close(c.events)
}
