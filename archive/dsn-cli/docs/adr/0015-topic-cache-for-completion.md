# Topic Cache for completion

Fish completion of Topic names reads a per-Profile local cache. The list-topics Query is the only refresh.

Live metadata on every TAB was rejected: completion can stampede the cluster. Stale names are acceptable; Peek by exact name does not need a cache hit. A cache shared across Profiles was rejected because two kafka Profiles are different clusters.
