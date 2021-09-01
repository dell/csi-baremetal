# Proposal: Validation webhook for CSI Deployment

Last updated: 01.09.2021


## Abstract

Add CSI Deployment Validation webhook in csi-baremetal Operator

## Background

Need to check that there is only one csi-baremetal instance on the cluster. 
In additional webhook helps to verify csi-deployment manifest before installing. 

## Proposal

### Validating webhook

1. Apply ValidatingWebhookConfiguration recourse. It must contain Certificate and link to service with endpoint.
2. Apply service with csi-baremetal-operator selector.
3. Add the opportunity to generate TLS certificates.
4. Implement webhook server: check CREATE request and reject it when another csi-baremetal exists.

### Certificates generating

#### Self-signed certificate

Kubernetes accepts self-signed certs for now. We can create it whit openssl or in code using crypto/x509.
This method isn't guarantee secure connection and can be deprecated in the future.

#### CertificateSigningRequest

The algorithm:
1. Generate `CERTIFICATE REQUEST` using openssl or crypto/x509.
2. Create CertificateSigningRequest (CSR)
3. Approve CSR
4. Get signed certificate from CSR
5. Certificate can be placed in secret to not update it if pod failed

Certificates can be rounded in Operator. 
After cert-duration will be ended Operator should recreate CSR, patch ValidatingWebhookConfiguration and restart server.

#### Cert-manager

The algorithm:
1. Create Issuer and Certificate cert-manager resources
2. Get signed certificate from the generated secret

Certificates will be rounded automatically by cert-manager.
Operator should check Secret, patch ValidatingWebhookConfiguration and restart server.

## Rationale

#### Alternative approach

Make CSI Deployment cluster scoped and don't create any new resources from different CSIs

## Compatibility

It has no compatibility issues 

## Implementation

Prototype implementation - https://github.com/dell/csi-baremetal-operator/blob/experiments-webhook/pkg/csiwebhook/cert.go

## Open issues (if applicable)

ID | Name | Descriptions | Status | Comments
---| -----| -------------| ------ | --------
ISSUE-1 | Cert-manager will be extra dependency  | Should we add cert-manager deploying into release notes?  |   |
ISSUE-2 | Deployment was created without Operator  | How we can forbidden CSI Deployment creation until Operator is not ready?   |   |   
