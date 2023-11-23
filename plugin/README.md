Plugins
---

This directory contains plug-in support for external libraries to use with a `Client`.

<pre>
<a href="./">plugin</a> — you are here
├── <a href="./rprom">rprom</a> — plug-in Prometheus metrics
├── <a href="./rotel">rotel</a> — plug-in OpenTelemetry tracing
├── <a href="./rlogrus">rlogrus</a> — plug-in sirupsen/logrus logging lib
└── <a href="./rzap">rzap</a> — plug-in uber-go/zap logging lib
</pre>

These plugins can be enabled by using [Client.WithHooks](https://pkg.go.dev/github.com/jamf/regatta-go?utm_source=godoc#WithHooks) function in your 
GO application. For example:

```
// Create the client
c, err := client.New(
    client.WithEndpoints("127.0.0.1:8443"),
    client.WithLogger(client.PrintLogger{}),
    client.WithSecureConfig(&client.SecureConfig{
        InsecureSkipVerify: true, // Skip verification of self-signed certificate
    }),
    client.WithHooks(rprom.NewMetrics(), rotel.NewTracing()), // Enable rprom and rotel plugins
)
if err != nil {
    panic(err)
}
```
