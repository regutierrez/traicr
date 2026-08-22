# Retain Source Records and normalize server-side

Trace ZIPs retain lossless harness-specific Source Records, while the server derives common Events for search and display. This uses more storage than keeping only normalized data, but it preserves information, keeps collector behavior simple, and allows Harness Normalizer fixes to rebuild the search index without recollecting traces from every source machine.
