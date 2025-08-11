# swaycycle

Cycle through  all visible windows  on a sway[fx]  workspace including
floating  ones or  windows  in sub-containers.   So  it simulates  the
behavior of other window managers  and desktop environments. Just bind
the tool to `ALT-tab` and there you go.

![Screenshot](https://github.com/TLINDEN/swaycycle/blob/main/.github/assets/screenshot.png)

## Installation

Download  the   binary  for   your  architecture  from   the  [release
page](https://github.com/TLINDEN/swaycycle/releases) and copy to to to
some location within your `$PATH`.

To  build  the  tool  from  source,  checkout  the  repo  and  execute
`make`. You'll need  the go toolkit. Then copy  the binary `swaycycle`
to some location within your `$PATH`.

## Configuration

Add such a line to your sway config file (e.g. in `$HOME/.config/sway/config`):

```default
bindsym $mod+Tab exec ~/bin/swaycycle
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

## Getting help

Although I'm happy to hear from swaycycle users in private email, that's the
best way for me to forget to do something.

In order to report a bug,  unexpected behavior, feature requests or to
submit    a    patch,    please    open   an    issue    on    github:
https://github.com/tlinden/swaycycle/issues.

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
