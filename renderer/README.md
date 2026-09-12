# Renderer definitions

Every renderer is described by `renderer/<id>/renderer.json`. The same discovery
code loads these shipped definitions and user-provided definitions from
`--renderer-dir`; an explicit directory wins when IDs overlap.

Runtime assets stay in the package-level `runtime/` directory. Manifests use
paths relative to their renderer directory and may use `platform_resources` for
Windows/Linux filenames. All definitions use manifest version 2. See
`docs/plugins.md` and `docs/renderer.schema.json`.

The application includes compiled adapters for the providers declared by the
shipped manifests. A manifest does not provide a general dynamic engine ABI.
