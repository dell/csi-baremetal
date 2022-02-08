# Proposal: Distributed tracing of CSI volume requests 

Last updated: 7-Feb-2022


## Abstract

Distributed tracing is a must for every microservices architecture: it allows to detect issues quickly and offers valuable insights that can decrease response time from hours to minutes.

## Background

To create, expand, delete persistent volumes different requests are handled by CSI across multiple components. For details please see [flow diagrams of CSI volume requests](https://github.com/dell/csi-baremetal/blob/master/docs/volume-requests-flow.md).
When system doesn't respond in expected time frame it's really hard to find the issue using just metrics or log analysis.

## Proposal

Proposal is to use OpenTelemetry API and SDK https://opentelemetry.io/docs/instrumentation/go/ and distributed tracing system Jaeger https://www.jaegertracing.io/ to:
- Distributed transaction monitoring
- Root cause analysis
- Performance / latency optimization

## Rationale

Distributed tracing must be supported in CSI. The open question whether to make it configurable or not.

## Compatibility

No compatibility issues

## Implementation

The following methods must be instrumented:
- Scheduler extender
  - FilterHandler
  - PrioritizeHandler
- Controller
  - Reconcile of ACR CR
  - CreateVolume
  - ControllerExpandVolume
  - DeleteVolume
- Node
  - Reconcile of Volume CR
  - Reconcile of LVG CR
  - NodeStageVolume
  - NodePublishVolume
  - NodeUnpublishVolume
  - NodeUnstageVolume

## Assumptions (if applicable)

ID | Name | Descriptions | Comments
---| -----| -------------| --------
ASSUM-1 |   |   |


## Open issues (if applicable)

ID | Name | Descriptions | Status | Comments
---| -----| -------------| ------ | --------
ISSUE-1 | Optional | Should tracing be optional or mandatory? | Open | Do we need it in production?
