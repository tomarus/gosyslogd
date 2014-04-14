This program receives syslog messages on tcp, udp or unix sockets.

It parses all messages according to a list of regular expressions.

Regex lists are stored in /etc/gosyslogd. The filename must be the same
as the syslog tag. Only tags which have a regex list are monitored.

Unmatches messages are published to a Redis channel "logging" and stored
in a PostgreSQL database in a table called "log_YYYYMM".

Matched messages are stored in a Mongo DB capped collection. The name
of the collection is a md5 sum of the regular expressions. Matched
messages which are marked as important are published to Redis
channel "critical".
