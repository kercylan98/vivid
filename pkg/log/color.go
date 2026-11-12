package log

import (
	"os"

	"github.com/kercylan98/vivid/pkg/log/internal/isatty"
)

var (
	noColor = os.Getenv("NO_COLOR") != "" || os.Getenv("TERM") == "dumb" ||
		(!isatty.IsTerminal(os.Stdout.Fd()) && !isatty.IsCygwinTerminal(os.Stdout.Fd()))
)
