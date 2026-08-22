# Collect locally with a manual client

Each source machine will have a standalone Traicr collector that is run manually to create and upload Trace ZIPs; it will not run as a daemon. Server-controlled SSH collection was rejected because it would give the archive server access to every source machine and make collection depend on remote availability, while local collection keeps file and harness credentials on the machine that owns them.
