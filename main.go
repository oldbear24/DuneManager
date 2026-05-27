package main

import (
"dune-manager/internal/config"
"dune-manager/internal/ui"
)

func main() {
config.Init()
ui.Run()
}
