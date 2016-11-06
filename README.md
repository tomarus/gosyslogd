This program receives syslog messages on tcp, udp or unix sockets.

It parses all messages according to a list of regular expressions.

Regex lists are stored in /etc/gosyslogd. The filename must be the same
as the syslog tag. Only tags which have a regex list are monitored.

Unmatched messages are published to a Redis channel "logging" and stored
in a PostgreSQL database in a table called "log_YYYYMM".

Matched messages which are marked as important are published to Redis
channel "critical".

There is a little web interface to monitor incoming syslog messages.

This was written a few years ago back in 2014 and is old code mostly.

