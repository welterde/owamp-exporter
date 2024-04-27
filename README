# owamp-exporter

owamp-exporter allows the one-way latency and loss measurement using the [OWAMP toolkit](https://github.com/perfsonar/owamp), which employes the One-way Active Measurement Protocol (OWAMP) as specified in [RFC 4656](https://datatracker.ietf.org/doc/html/rfc4656).
This exporter supports multiple measurement pairings and also allows measurements between pairs where neither node has to be node running the exporter.
Under the hood `powstream` is employed which performs continous one-way measurements.
The output summary files are parsed and are exposed in this exporter.

## Usage

```
Usage of ./owamp-exporter:
  -cfg-file string
    	The configuration file (default "owamp-export.cfg")
  -listen-port uint
    	Listen port for exporter (default 9099)
  -powstream-cmd string
    	Location of powstream binary to use (default "powstream")
  -victoria-histogram
    	Use the VictoriaMetrics histogram format
  -workdir string
    	Location to place collected owping reports
```

## Configuration

The configuration is fairly straight-forward: One first designs the measurement nodes (one of which can be the local host, but does not need to be) and then defines all the desired measurement pairings with a target packet-per-second (PPS) value for each.

Short example Configuration:

```
# SYNTAX: TARGET <name> <hostname> [options]
TARGET tgt1 localhost local
TARGET tgt2 2001:db8::1

# SYNTAX: MEASUREMENT <name1> <name2> [options in key=value syntax]
# packets are sent from node 1 in the direction of node 2
MEASUREMENT tgt1 tgt2
MEASUREMENT tgt2 tgt1
```

A more detailed configuration file with all the other options explained can be found [here](example_config.txt)


## Metrics

The exporter produces the following metrics:

- `owamp_start_time`: UNIX timestamp of the start time of the measurement session
- `owamp_end_time`: UNIX timestamp of the end time of the measurement session
- `owamp_packets_sent`: Number of packets sent during the measurement session
- `owamp_packets_dup`: Number of duplicate packets received during the measurement session
- `owamp_packets_lost`: Number of packets lost during the measurement session
- `owamp_latency_bucket`: One-way latency histogram during the measurement session
- `owamp_latency_sum`: Cumulative sum of the latency histogram
- `owamp_latency_count`: Number of samples in the latency histogram
- `owamp_latency_min`: Minimum one-way latency during measurement session
- `owamp_latency_median`: Median one-way latency during measurement session
- `owamp_latency_max`: Maximum one-way latency during measurement session
- `owamp_ttl_bucket`: Packet TTL histogram during measurement session
- `owamp_ttl_sum`: Cumulative sum of the TTL histogram
- `owamp_ttl_count`: Number of samples in the TTL histogram
- `owamp_reordering_bucket`: Histogram of the number of reordering events during the measurement session
- `owamp_reordering_sum`: Cumulative sum of reordering events
- `owamp_reordering_count`: Number of samples in the reordering histogram
- `owamp_time_error_estimate`: Estimate of the uncertainty of the absolute time calibration

All metrics are emitted with the timestamp set to the mid-point of the last measurement session.
The reordering histogram is only emitted if reordering events are detected.
Depending if `-victoria-histogram` is set or not the histograms are emitted in the prometheus format (with the bins set by the binwidth set in the configuration) or the victoriametrics histogram format.

Since multiple measurement pairs can be run by this exporter all of the above metrics include the following labels:

- `src_short_name`: The short-name of the origin node of the one-way measurement
- `dst_short_name`: The short-name of the destination node of the one-way measurement
- `src_hostname`: The hostname of the origin node
- `dst_hostname`: The hostname of the destination node
- `afi`: Address Family used in the measurement (either ip6 for IPv6 or ip4 for IPv4)
