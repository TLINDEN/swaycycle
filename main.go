/*
Copyright © 2025 Thomas von Dein

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU General Public License for more details.

You should have received a copy of the GNU General Public License
along with this program. If not, see <http://www.gnu.org/licenses/>.
*/

package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"log/slog"
	"os"
	"os/exec"
	"runtime/debug"

	"github.com/lmittmann/tint"
	"github.com/mattn/go-isatty"
	"github.com/tlinden/yadu"

	flag "github.com/spf13/pflag"
)

type Node struct {
	Id                int    `json:"id"`
	Nodetype          string `json:"type"` // output, workspace or container
	Name              string `json:"name"` // workspace number or app name
	Nodes             []Node `json:"nodes"`
	FloatingNodes     []Node `json:"floating_nodes"`
	Focused           bool   `json:"focused"`
	Window            int    `json:"window"` // wayland native
	X11Window         string `json:"app_id"` // x11 compat
	Current_workspace string `json:"current_workspace"`
}

const (
	root = iota + 1
	output
	workspace
	con
	floating

	LevelNotice = slog.Level(2)

	VERSION = "v0.1.2"
)

var (
	Visibles         = []Node{}
	CurrentWorkspace = ""
	Debug            = false
	Dumptree         = false
	Version          = false
	Verbose          = false
	Notswitch        = false
	Showhelp         = false
	Logfile          = ""
)

const Usage string = `This is swaycycle - cycle focus through all visible windows on a sway workspace.

Usage: swaycycle [-vVdDn] [-l <log>]

Options:
  -n, --no-switch        do not switch windows
  -d, --debug            enable debugging
  -D, --dump             dump the sway tree (needs -d as well)
  -l, --logfile string   write output to logfile
  -v, --verbose          enable verbose logging
  -V, --version          show program version

Copyleft (L) 2025 Thomas von Dein.
Licensed under the terms of the GNU GPL version 3.
`

func main() {
	flag.BoolVarP(&Debug, "debug", "d", false, "enable debugging")
	flag.BoolVarP(&Dumptree, "dump", "D", false, "dump the sway tree (needs -d as well)")
	flag.BoolVarP(&Verbose, "verbose", "v", false, "enable verbose logging")
	flag.BoolVarP(&Notswitch, "no-switch", "n", false, "do not switch windows")
	flag.BoolVarP(&Version, "version", "V", false, "show program version")
	flag.BoolVarP(&Showhelp, "help", "h", Showhelp, "show help")

	flag.StringVarP(&Logfile, "logfile", "l", "", "write output to logfile")
	flag.Parse()

	if Version {
		fmt.Printf("This is swaycycle version %s\n", VERSION)
		os.Exit(0)
	}

	if Showhelp {
		fmt.Println(Usage)
		os.Exit(0)
	}

	// setup logging
	if Logfile != "" {
		file, err := os.OpenFile(Logfile, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0600)
		if err != nil {
			log.Fatalf("Failed to open logfile %s: %s", Logfile, err)
		}
		defer file.Close()
		setupLogging(file)
	} else {
		setupLogging(os.Stdout)
	}

	// fills Visibles node list
	fetchSwayTree()

	if len(Visibles) == 0 {
		os.Exit(0)
	}

	id := findNextWindow()

	if id > 0 && !Notswitch {
		switchFocus(id)
	}
}

func setupLogging(output io.Writer) {
	logLevel := &slog.LevelVar{}

	if !Debug {
		// default logging
		opts := &tint.Options{
			Level:     logLevel,
			AddSource: false,
			NoColor:   IsNoTty(),
		}

		if Verbose {
			logLevel.Set(slog.LevelInfo)
		} else {
			logLevel.Set(LevelNotice)
		}

		handler := tint.NewHandler(output, opts)
		logger := slog.New(handler)

		slog.SetDefault(logger)
	} else {
		// we're using a more verbose logger in debug mode
		buildInfo, _ := debug.ReadBuildInfo()
		opts := &yadu.Options{
			Level:     logLevel,
			AddSource: true,
		}

		logLevel.Set(slog.LevelDebug)

		handler := yadu.NewHandler(output, opts)
		debuglogger := slog.New(handler).With(
			slog.Group("program_info",
				slog.Int("pid", os.Getpid()),
				slog.String("go_version", buildInfo.GoVersion),
			),
		)
		slog.SetDefault(debuglogger)
	}
}

// find the next window after the  one with current focus. if the last
// one has focus, return the first
func findNextWindow() int {
	if len(Visibles) == 0 {
		return 0
	}

	seenfocused := false

	for _, node := range Visibles {
		if node.Focused {
			seenfocused = true
			continue
		}

		if seenfocused {
			return node.Id
		}
	}

	if seenfocused {
		return Visibles[0].Id
	}

	return 0
}

// actually switch focus using a swaymsg command
func switchFocus(id int) {
	var cmd *exec.Cmd
	arg := fmt.Sprintf("[con_id=%d]", id)

	slog.Debug("executing", "command", "swaymsg "+arg+" focus")

	cmd = exec.Command("swaymsg", arg, "focus")

	errbuf := &bytes.Buffer{}
	cmd.Stderr = errbuf

	out, err := cmd.Output()

	if err != nil {
		slog.Debug("failed to execute swaymsg", "output", out)
		log.Fatalf("Failed to execute swaymsg to switch focus: %s", err)
	}

	if errbuf.String() != "" {
		log.Fatalf("swaymsg error: %s", errbuf.String())
	}

	slog.Info("switched focus", "con_id", id)
}

// execute swaymsg to get its internal tree
func fetchSwayTree() {
	var cmd *exec.Cmd

	slog.Debug("executing", "command", "swaymsg -t get_tree -r")

	cmd = exec.Command("swaymsg", "-t", "get_tree", "-r")

	errbuf := &bytes.Buffer{}
	cmd.Stderr = errbuf

	out, err := cmd.Output()

	if err != nil {
		log.Fatalf("Failed to execute swaymsg to get json tree: %s", err)
	}

	if errbuf.String() != "" {
		log.Fatalf("swaymsg error: %s", errbuf.String())
	}

	if err := processJSON(out); err != nil {
		log.Fatalf("%s", err)
	}
}

func istype(nd Node, which int) bool {
	switch nd.Nodetype {
	case "root":
		return which == root
	case "output":
		return which == output
	case "workspace":
		return which == workspace
	case "con":
		return which == con
	case "floating_con":
		return which == floating
	}

	return false
}

// get into the sway tree, determine current workspace and extract all
// its visible windows, store them in the global var Visibles
func processJSON(jsoncode []byte) error {
	sway := Node{}

	if err := json.Unmarshal(jsoncode, &sway); err != nil {
		return fmt.Errorf("Failed to unmarshal json: %w", err)
	}

	if !istype(sway, root) && len(sway.Nodes) == 0 {
		return fmt.Errorf("Invalid or empty JSON structure")
	}

	if Dumptree {
		slog.Debug("processed sway tree", "sway", sway)
	}

	for _, node := range sway.Nodes {
		if node.Current_workspace != "" {
			// this is an output node containing the current workspace
			CurrentWorkspace = node.Current_workspace
			recurseNodes(node.Nodes)
			break
		}
	}

	slog.Debug("processed visible windows", "visibles", Visibles)

	return nil
}

// iterate recursively over given node list extracting visible windows
func recurseNodes(nodes []Node) {
	for _, node := range nodes {
		// we handle nodes and floating_nodes identical
		node.Nodes = append(node.Nodes, node.FloatingNodes...)

		if istype(node, workspace) {
			if node.Name == CurrentWorkspace {
				recurseNodes(node.Nodes)
				return
			}

			// ignore other workspaces
			continue
		}

		// the first nodes seen are workspaces, so if we see a con
		// node, we are already inside the current workspace
		if (istype(node, con) || istype(node, floating)) &&
			(node.Window > 0 || node.X11Window != "") {
			Visibles = append(Visibles, node)
		} else {
			recurseNodes(node.Nodes)
		}
	}
}

// returns TRUE if stdout is NOT a tty or windows
func IsNoTty() bool {
	if !isatty.IsTerminal(os.Stdout.Fd()) {
		return true
	}

	// it is a tty
	return false
}
