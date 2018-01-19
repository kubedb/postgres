## pg-operator run

Run Postgres in Kubernetes

### Synopsis

Run Postgres in Kubernetes

```
pg-operator run [flags]
```

### Options

```
      --address string                   Address to listen on for web interface and telemetry. (default ":8080")
      --docker-registry string           User provided docker repository (default "kubedb")
      --exporter-tag string              Tag of kubedb/operator used as exporter (default "canary")
      --governing-service string         Governing service for database statefulset (default "kubedb")
  -h, --help                             help for run
      --kubeconfig string                Path to kubeconfig file with authorization information (the master location is set by the master flag).
      --master string                    The address of the Kubernetes API server (overrides any value in kubeconfig)
      --prometheus-crd-apigroup string   prometheus CRD  API group name (default "monitoring.coreos.com")
      --prometheus-crd-kinds CrdKinds     - EXPERIMENTAL (could be removed in future releases) - customize CRD kind names
      --rbac                             Enable RBAC for database workloads
```

### Options inherited from parent commands

```
      --alsologtostderr                  log to standard error as well as files
      --analytics                        Send analytical events to Google Analytics (default true)
      --log.format logFormatFlag         Set the log target and format. Example: "logger:syslog?appname=bob&local=7" or "logger:stdout?json=true" (default "logger:stderr")
      --log.level levelFlag              Only log messages with the given severity or above. Valid levels: [debug, info, warn, error, fatal] (default "info")
      --log_backtrace_at traceLocation   when logging hits line file:N, emit a stack trace (default :0)
      --log_dir string                   If non-empty, write log files in this directory
      --logtostderr                      log to standard error instead of files
      --stderrthreshold severity         logs at or above this threshold go to stderr
  -v, --v Level                          log level for V logs
      --vmodule moduleSpec               comma-separated list of pattern=N settings for file-filtered logging
```

### SEE ALSO

* [pg-operator](pg-operator.md)	 - 

