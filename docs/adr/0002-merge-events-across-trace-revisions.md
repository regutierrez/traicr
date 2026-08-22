# Merge Events across trace revisions

The Trace Archive identifies traces by harness and native trace ID, then merges unique Events observed across every revision instead of selecting the most recently imported revision. This prevents a stale source machine from replacing a more complete history, while retained Source Records preserve conflicting or changed native data.
