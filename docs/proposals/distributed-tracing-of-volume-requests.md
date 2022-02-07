# Proposal: Distributed tracing of CSI volume requests 

Last updated: 7-Feb-2022


## Abstract

Distributed tracing is a must for every microservices architecture: it allows to detect issues quickly and offers valuable insights that can decrease response time from hours to minutes.

## Background

To create, expand, delete persistent volumes different requests are handled by CSI in the different components. For details please see ../docs/volume-requests-flow.md.
When system doesn't respond in expected time frame it's really hard to find the issue using just metrics or log analysis.

## Proposal

Proposal is to use OpenTelemetry API and SDK https://opentelemetry.io/docs/instrumentation/go/ and distributed tracing system Jaeger https://www.jaegertracing.io/ to:
- Distributed transaction monitoring
- Root cause analysis
- Performance / latency optimization

## Rationale

[A discussion of alternate approaches and the trade offs, advantages, and disadvantages of the specified approach.]

## Compatibility

[A discussion of the change with regard to the compatibility with previous product and Kubernetes versions.]

## Implementation

[A description of the steps in the implementation, who will do them, and when.]

## Assumptions (if applicable)

ID | Name | Descriptions | Comments
---| -----| -------------| --------
ASSUM-1 |   |   |


## Open issues (if applicable)

ID | Name | Descriptions | Status | Comments
---| -----| -------------| ------ | --------
ISSUE-1 |   |   |   |   
