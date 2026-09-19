# Anchors are collected into the archive, not written to the repository

A Trace can be linked to the commits it produced, but that link is an Anchor stored in the archive. The collector never installs a Git notes ref that would be pushed, and it never writes hook state that collaborators would see.

On a Git working copy, collect-time `git log` in the Trace's working directory supplies candidate commits in the session window. On a Jujutsu working copy, `jj log` supplies only described revisions in that window (not the live working-copy commit), and the Anchor records both the Git SHA and the Jujutsu change-id. Git hooks are not used for Jujutsu because Jujutsu does not run them.

Existing traces gain Patches and Signals from a normalizer rebuild of retained Source Records. They gain Anchors only when recollected from a machine that still has that Repository's history.
