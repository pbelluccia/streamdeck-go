# Third-party notices

## Fluent UI Stream Deck icon pack

`setup.sh` downloads the Fluent UI Stream Deck icon pack from:

https://github.com/czottmann/streamdeck-iconpack-fluentui-system-icons

The icon pack is based on Microsoft's Fluent UI System Icons:

https://github.com/microsoft/fluentui-system-icons

## Go modules

The Go module uses:

- `golang.org/x/image`
- `golang.org/x/text`

Both are distributed by the Go project under a BSD-style license.

The application code in this repository does not vendor third-party dependencies. Downloaded icon assets are installed into `~/.local/share/streamdeck-go/icons`.
