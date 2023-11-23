Prometheus plugin
---

Provides [Prometheus](https://prometheus.io) metrics for Regatta Go client.

### Metrics

* `<namespace>[_subsystem]_connects_total` - Total number of connections opened
* `<namespace>[_subsystem]_disconnects_total` - Total number of connections closed
* `<namespace>[_subsystem]_connect_active` - Number of active connections
* `<namespace>[_subsystem]_write_bytes_total` - Total number of bytes written
* `<namespace>[_subsystem]_read_bytes_total` - Total number of bytes read
* `<namespace>[_subsystem]_request_duration_seconds{table,op}` - Time histogram spent executing KV operation per table and operation
* `<namespace>[_subsystem]_request_errors_total{table,op}` - Total number of errors while executing KV operation per table and operation
