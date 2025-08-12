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
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"log/slog"
	"net"
	"os"
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

type Response struct {
	Success    bool   `json:"success"`
	ParseError bool   `json:"parse_error"`
	Error      string `json:"error"`
}

const (
	root = iota + 1
	output
	workspace
	con
	floating

	LevelNotice = slog.Level(2)

	VERSION = "v0.2.0"

	IPC_HEADER_SIZE = 14
	IPC_MAGIC       = "i3-ipc"

	// message types
	IPC_GET_TREE    = 4
	IPC_RUN_COMMAND = 0
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

Usage: swaycycle [-vdDn] [-l <log>]

Options:
  -n, --no-switch        do not switch windows
  -d, --debug            enable debugging
  -D, --dump             dump the sway tree (needs -d as well)
  -l, --logfile string   write output to logfile
  -v, --version          show program version

Copyleft (L) 2025 Thomas von Dein.
Licensed under the terms of the GNU GPL version 3.
`

func main() {
	flag.BoolVarP(&Debug, "debug", "d", false, "enable debugging")
	flag.BoolVarP(&Dumptree, "dump", "D", false, "dump the sway tree (needs -d as well)")
	flag.BoolVarP(&Notswitch, "no-switch", "n", false, "do not switch windows")
	flag.BoolVarP(&Version, "version", "v", false, "show program version")
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

	// connect to sway unix socket
	unixsock, err := setupIPC()
	if err != nil {
		log.Fatalf("Failed to connect to sway unix socket: %s", err)
	}

	// retrieve the raw json tree
	rawjson, err := getTree(unixsock)
	if err != nil {
		log.Fatalf("Failed to retrieve raw json tree: %s", err)
	}

	// traverse the tree and find visible windows
	if err := processJSON(rawjson); err != nil {
		log.Fatalf("%s", err)
	}

	if len(Visibles) == 0 {
		os.Exit(0)
	}

	id := findNextWindow()
	slog.Debug("findNextWindow", "nextid", id)

	if id > 0 && !Notswitch {
		switchFocus(id, unixsock)
	}
}

// connect to unix socket
func setupIPC() (net.Conn, error) {
	sockfile := os.Getenv("SWAYSOCK")

	if sockfile == "" {
		return nil, fmt.Errorf("Environment variable SWAYSOCK does not exist or is empty")
	}

	conn, err := net.Dial("unix", sockfile)
	if err != nil {
		return nil, err
	}

	return conn, nil
}

// send a sway message header
func sendHeaderIPC(sock net.Conn, messageType uint32, len uint32) error {
	sendPayload := make([]byte, IPC_HEADER_SIZE)
	copy(sendPayload, []byte(IPC_MAGIC))
	binary.LittleEndian.PutUint32(sendPayload[6:], len)
	binary.LittleEndian.PutUint32(sendPayload[10:], messageType)

	_, err := sock.Write(sendPayload)

	if err != nil {
		return fmt.Errorf("failed to send header to IPC %w", err)
	}

	return nil
}

// send a payload, header had to be sent before
func sendPayloadIPC(sock net.Conn, payload []byte) error {
	_, err := sock.Write(payload)

	if err != nil {
		return fmt.Errorf("failed to send payload to IPC %w", err)
	}

	return nil
}

// read a response, reads response header and returns payload only
func readResponseIPC(sock net.Conn) ([]byte, error) {
	// read header
	buf := make([]byte, IPC_HEADER_SIZE)

	_, err := sock.Read(buf)
	if err != nil {
		return nil, fmt.Errorf("failed to read header from socket: %s", err)
	}

	slog.Debug("got IPC header", "header", hex.EncodeToString(buf))

	if string(buf[:6]) != IPC_MAGIC {
		return nil, fmt.Errorf("got invalid IPC response from sway socket")
	}

	payloadLen := binary.LittleEndian.Uint32(buf[6:10])

	if payloadLen == 0 {
		return nil, fmt.Errorf("got empty payload IPC response from sway socket")
	}

	// read payload
	payload := make([]byte, payloadLen)

	_, err = sock.Read(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to read payload from socket: %s", err)
	}

	return payload, nil
}

// get raw JSON tree via sway IPC
func getTree(sock net.Conn) ([]byte, error) {
	err := sendHeaderIPC(sock, IPC_GET_TREE, 0)
	if err != nil {
		return nil, err
	}

	payload, err := readResponseIPC(sock)
	if err != nil {
		return nil, err
	}

	return payload, nil
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
func switchFocus(id int, sock net.Conn) error {
	command := fmt.Sprintf("[con_id=%d] focus", id)

	slog.Debug("executing", "command", command)

	// send switch focus command
	err := sendHeaderIPC(sock, IPC_RUN_COMMAND, uint32(len(command)))
	if err != nil {
		return err
	}

	if err != nil {
		return fmt.Errorf("failed to send run_command to IPC %w", err)
	}

	err = sendPayloadIPC(sock, []byte(command))
	if err != nil {
		return fmt.Errorf("failed to send switch focus command: %w", err)
	}

	// check response from sway
	payload, err := readResponseIPC(sock)
	if err != nil {
		return err
	}

	responses := []Response{}

	if err := json.Unmarshal(payload, &responses); err != nil {
		return fmt.Errorf("Failed to unmarshal json response: %w", err)
	}

	if len(responses) == 0 {
		return fmt.Errorf("Got invalid IPC zero response")
	}

	if !responses[0].Success {
		slog.Debug("IPC response to switch focus command", "response", responses)
		return fmt.Errorf("Failed to switch focus: %s", responses[0].Error)
	}

	slog.Info("switched focus", "con_id", id)

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

// we use line wise logging, unless debugging is enabled
func setupLogging(output io.Writer) {
	logLevel := &slog.LevelVar{}

	if !Debug {
		// default logging
		opts := &tint.Options{
			Level:     logLevel,
			AddSource: false,
			NoColor:   IsNoTty(),
		}

		logLevel.Set(slog.LevelInfo)

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

// little helper to distinguish sway tree node types
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

// returns TRUE if stdout is NOT a tty or windows
func IsNoTty() bool {
	if !isatty.IsTerminal(os.Stdout.Fd()) {
		return true
	}

	// it is a tty
	return false
}
