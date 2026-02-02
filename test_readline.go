package main

import (
	"fmt"
	"github.com/chzyer/readline"
)

func main() {
	rl, err := readline.New("> ")
	if err != nil {
		panic(err)
	}
	defer rl.Close()

	for {
		line, err := rl.Readline()
		if err != nil {
			break
		}
		fmt.Println("You entered:", line)
		if line == "exit" {
			break
		}
	}
}
