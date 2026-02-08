package main

import (
	"github.com/MSmaili/muxie/cmd"

	_ "github.com/MSmaili/muxie/internal/backend/tmux"
)

func main() {
	cmd.Execute()
}
