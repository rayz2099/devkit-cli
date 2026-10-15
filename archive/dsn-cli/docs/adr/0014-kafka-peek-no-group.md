# Kafka Peek does not join a consumer group

A kafka Query that reads Records assigns partitions and never joins a consumer group or commits offsets.

Joining a group was rejected: lag monitors treat the CLI as a consumer and alert. Resume-from-group is a Write, not a Peek, and is out of this Kind's read surface.
