# The browser page

FleetWatch serves a page that renders the board live. Open `http://127.0.0.1:8909/`, paste the token once, and the sessions appear. The token is `FLEET_TOKEN` when set, else the generated one stored at `$XDG_STATE_HOME/fleet/token` (the startup log names the path). Authentication is described in ADR-012 and `docs/security.md`.

## Routes

| Route | Auth | Purpose |
|---|---|---|
| `GET /` | none | the page |
| `GET /ui/{index.html,fleet.js,fleet.css,theme.css}` | none | the page's assets |
| `POST /v1/session` | the token in the body, and a same-host `Origin` | log in; sets the session cookie |
| `DELETE /v1/session` | a same-host `Origin` | log out |
| `GET /v1/fleet`, `GET /v1/events` | bearer token or session cookie | the board, as JSON and as Server-Sent Events |

`ui.enabled: false` removes the first four rows: `/` and `/v1/session` answer 404.

## Theme

The page is two stylesheets. `fleet.css` is structure: layout and components, with every colour and font taken from a `--fl-*` custom property. `theme.css` gives those properties their values and maps each model class to a fill. A model's class is the `class` of the matching `models` entry in the config, carried on the board as `model_class`; a class with no rule in the theme is drawn in `--fl-mc-other`.

To change the look, copy the built-in `theme.css` (`web/theme.css` in the source, or `GET /ui/theme.css` from a running instance), edit it and name it in the config:

```json
{ "version": 1, "ui": { "theme_file": "~/.config/fleet/theme.css" } }
```

The file is checked and read once at start, so restart to apply an edit. It must be a regular file (a symlink is refused) with a `.css` extension and at most 512 KiB; if it fails any check, the built-in theme is served and the reason is logged. The file is a stylesheet and nothing else: the page's policy allows no inline script, and `fleet.css` is not replaceable.

## Source and build

The page is plain JavaScript in `web/`, with no dependencies and no bundler. Run its tests with `node --test web/*.test.js`. `node web/build.mjs` writes `internal/ui/dist/` (`index.html`, `fleet.js`, `fleet.css`, `theme.css`), which is committed and embedded into the binary, so `go build` needs no Node. CI rebuilds it and fails when the committed output differs. The layout test in `web/` drives headless Chrome when one is installed (`CHROME_BIN` or `/usr/bin/google-chrome`) and is skipped otherwise.

The library is also packaged as `@fleetwatch/ui` (`web/package.json`) for a host that wants the render functions and page shell without the Go server. `fleetPageHTML` builds either the linked page FleetWatch serves or a single document with its script and styles inlined under a nonce.
