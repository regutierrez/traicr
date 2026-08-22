# Use Go for the collector and server

The collector and server will be written in Go rather than copying the reference extractors' Python structure. The collector will ship as standalone macOS and Linux binaries that do not require Go on source machines, while the server ships as a small Docker image; both share Trace ZIP types and one implementation language across collection, normalization, storage, API, and Web UI.
