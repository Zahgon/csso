# csso 5.0.5 in Go: what changed

The JavaScript package (`lib/`, `dist/`, `scripts/`, `test/`, `package.json`, `package-lock.json`, the
ESLint configuration and the Node and GitHub Pages workflows) is retired. The module
`github.com/css/csso-go` reimplements csso 5.0.5 together with what it inherits from css-tree 2.2.0
(tokenizer, parser and its error recovery, generator with csso's string and url forms, source maps
byte-identical to source-map-js 1.0.1, walker, List, keyword and property helpers) and css-tree's lexer
over mdn-data 2.0.28 with css-tree's data patch, which csso consults when it rewrites colours and
percentages. `fixtures/` keeps csso's test stylesheets; `docs/` keeps the descriptions of the
optimisations.

## API

- `Minify`, `MinifyBlock` and `Compress` take `Options`; `Restructure` is a `*bool` (nil means true, as
  the JavaScript default), `Comments` is one of `CommentsExclamation` (the default),
  `CommentsFirstExclamation` and `CommentsNone`, and `Usage` mirrors the usage object with nil slices
  meaning "not given".
- Exceptions csso throws come back as a `*csso.Error` whose `Error()` is the JavaScript
  `name + ": " + message`.
- `Parse` returns an `*AST`; `JSON()` is `JSON.stringify(ast)` of a parsed tree, without the
  annotations csso's compression writes onto nodes.
- The `debug` option is not provided (it writes timings to stderr); `Logger` receives the same titles.

## Remaining differences

- When a parse is given `Line` above 1 and an error falls past the source's own line count, css-tree
  throws a TypeError while building the error excerpt; the Go parser reports the syntax error itself.
  csso never sets `line`.
- Hash values named like `Object.prototype` members (for example `#constructor`) are treated as
  ordinary colours; the JavaScript lookup table inherits those names.
- With `Logger` set, the logged text for a root without children (`rule`, `atrule` or `declaration`
  contexts) is not identical to csso's.
