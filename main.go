package main

import (
"github.com/oldbear24/DuneManager/internal/config"
"github.com/oldbear24/DuneManager/internal/ui"
)

func main() {
config.Init()
ui.Run()
}
