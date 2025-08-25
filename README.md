# swaycycle

Cycle through  all visible windows  on a sway[fx]  workspace including
floating  ones or  windows  in sub-containers.   So  it simulates  the
behavior of other window managers  and desktop environments. Just bind
the tool to `ALT-tab` and there you go.


## Installation

Download  the   binary  for   your  architecture  from   the  [release
page](https://github.com/TLINDEN/swaycycle/releases) and copy to
some location within your `$PATH`.

To  build  the  tool  from  source,  checkout  the  repo  and  execute
`make`. You'll need  the go toolkit. Then copy  the binary `swaycycle`
to some location within your `$PATH`.

## Configuration

Add such a line to your sway config file (e.g. in `$HOME/.config/sway/config`):

```default
bindsym $mod+Tab exec ~/bin/swaycycle
```

You may  also add  a second key  binding to do  the reverse,  which is
sometimes very useful:

```default
bindsym $mod+Shift+Tab exec ~/bin/swaycycle --prev
```

## Debugging

You may call `swaycycle` in a terminal window on a workspace with at
least one another window to test it. Use the option `--debug (-d)` to
get comprehensive debugging output. Add the option `--dump (-D)` to
also get a dump of the sway data tree retrieved by swaycycle. You may
also try `--verbose (-v)` to get a oneliner about the switch.

It's also possible to debug an instance executed by sway using the
`--logfile (-l)` switch, e.g.:

```default
bindsym $mod+Tab exec ~/bin/swaycycle -d -l /tmp/cycle.log
```

## How does it work?

`swaycycle` is being executed by sway when the user presses a key
(e.g. `ALT-tab`). It then connects to the running sway instance via
the provided IPC unix domain socket as available in the environment
variable `SWAYSOCK`. Via that connection it sends the `GET_TREE`
command and processes the retrieved JSON response. This JSON tree
contains all information about the running instance such as outputs,
workspaces and containers.

Then it determines which workspace is the current active one and
builds a list of all windows visible on that workspace, whether
floating or not.

Next it determines which window is following the one in the list with
the current active focus. If the active one is at the end of the list,
it starts from the top.

Finally `swaycycle` sends the propper switch focus command via the IPC
connection to sway, e.g.:

`[con_id=14] focus`

## Getting help

Although I'm happy to hear from swaycycle users in private email, that's the
best way for me to forget to do something.

In order to report a bug,  unexpected behavior, feature requests or to
submit    a    patch,    please    open   an    issue    on    github:
https://github.com/tlinden/swaycycle/issues.

## See also

- [sway-ipc(7)](https://www.mankier.com/7/sway-ipc)
- [swaywm](https://github.com/swaywm/sway/)
- [swayfx](https://github.com/WillPower3309/swayfx)

## Copyright and license

This software is licensed under the GNU GENERAL PUBLIC LICENSE version 3.

## Authors

T.v.Dein <tom AT vondein DOT org>

## Project homepage

https://github.com/tlinden/swaycycle

## Copyright and License

Licensed under the GNU GENERAL PUBLIC LICENSE version 3.

## Author

T.v.Dein <tom AT vondein DOT org>
