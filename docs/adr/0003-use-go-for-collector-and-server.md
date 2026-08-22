# Use Go for the collector and server

The collector and server will be written in Go rather than copying the reference extractors' Python structure. Go supports dependency-free collector binaries for macOS and Linux, a small Docker server image, shared Trace ZIP types, and one implementation language across collection, normalization, storage, API, and Web UI.
