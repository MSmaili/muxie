package main

import (
	"github.com/MSmaili/hetki/cmd"

	_ "github.com/MSmaili/hetki/internal/backend/tmux"
)

func main() {
	cmd.Execute()
}
