# midiloop

A SMF looper using an external clock. Patterns are selected based on midi song select messages; song 0 maps to `0.mid`, song 1 maps to `1.mid`, and so on in the loop bank directory. If the pattern file changes, `midiloop` will reload it. Intended to function in tandem with `midirec`.