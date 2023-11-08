Plugins
---

This directory contains plug-in support for external libraries to use with a
`Client`. The metrics here try to take a pragmatic approach to which
metrics are valuable while avoiding too much cardinality explosion, and the
logging packages try to do the right thing.

<pre>
<a href="./">plugin</a> — you are here
├── <a href="./rprom">rprom</a> — plug-in prometheus metrics
├── <a href="./rotel">rotel</a> — plug-in opentelemetry tracing
├── <a href="./rlogrus">rlogrus</a> — plug-in sirupsen/logrus logging lib
└── <a href="./rzap">rzap</a> — plug-in uber-go/zap logging lib
</pre>
