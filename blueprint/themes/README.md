# Themes

Each `{name}.json` is a palette: `name`, `base` (`dark`|`light`), and an `overrides` map of Claude Code UI color keys to hex values.
pfm embeds these palettes into the binary at build time; adding a file here ships a new selectable theme on the next build.
