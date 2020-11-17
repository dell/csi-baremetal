# Proposal: Metrics proposal

Last updated: 17.11.2020


## Abstract

Expose metrics of the baremetal CSI plugin that will give an idea of how different parts of the system performs.

## Background
As part of scalability testing, we should understand what parts are much slower and easily find the bottlenecks. After instrumenting the baremetal CSI plugin with metrics we'll be able to see hard it is working and what it's actually doing.

## Proposal

Expose metrics in Prometheus format with HTTP server at "/metrics" endpoint on every component:
 - Node
 - Controller  
 - Scheduler Extender    
 - Drive manager 

The HTTP server will be optional since it can cause some security issues(i.e. vulnerabilities with HTTP server)
Default port: 8888

Metrics:

Metric name                           | Metric type |                          Labels/tags                                     |              Description
 ------------------------------------ | ----------- | ------------------------------------------------------------------------ | ---------------------------------------
build_info                            | Gauge       | branch=\<git branch><br />revision=\<git rev><br />version=\<csi version>| information of the source code and driver
discovery_duration_seconds            | Histogram   | none                                                                     | duration of the discovery method for the drive manager
discovery_drive_count                 | Gauge       | none                                                                     | last drive count discovered
grpc_request_duration_seconds         | Histogram   | handler=\<grpc handler><br />error=\<error type>                         | duration of the request to grpc handlers
reconcile_duration_seconds            | Histogram   | type=\<type of reconcile>                                                | duration of the each reconcile loop. example of type -  "volume_manager"
volumes_operation_duration_seconds    | Histogram   | method=\<method name>                                                    | duration of operations on volumes
partitions_operation_duration_seconds | Histogram   | method=\<method name>                                                    | duration of operations on partitions
kubeclient_execution_duration_seconds | Histogram   | method=\<method name>                                                    | duration of kubectl methods
util_execution_duration_seconds       | Histogram   | name=\<util name><br />method=\<method name>                             | duration of the differents utils we use i.e. "lvm"
http_request_duration_seconds         | Histogram   | path=\<url path><br />code=\<http response code>                         | duration of the http requests

As I mentioned earlier, metrics will be exposed in Prometheus format and they can be consumed by any monitoring system like Prometheus, Telegraf, etc.

## Rationale

### Monitoring model
There are 2 ways to make monitoring of the application: Push or Pull metrics.
The push model obligates the application to send metrics to some kind of endpoint, and we don't want that.
What if don't have this collector yet, but wanted to check some measurements by hand? Or we don't need all metrics
From this side - The pull model is more preferable to use. We don't need to think about monitoring infrastructure. It could be 1 Prometheus application or HA cluster of Prometheus, or even your own hand-written monitoring system.
So it makes sense to let the user decide how and what he will store and visualize. The application area of ​​responsibility ends on publishing the "/metrics" endpoint.


## Compatibility

There are no issues with compatibility.

## Implementation

### Code changes
- New feature flag:
  - Enable webserver
  - Set webserver port
  - Set path for the metrics
- Wrap current functions with metrics if it's needed


### Helm charts:
The new feature flag will:
- Add feature specific parameters to the containers args
- Helm variable with HTTP port, and path for the metric
- Add Prometheus annotations to the components
```
annotations:
        prometheus.io/scrape: 'true'
        prometheus.io/port: 'port_number'
```
