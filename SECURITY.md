# Security policy

## What qualifies as a security issue

We consider a security vulnerability a software issue that compromises one or more of:

- Confidentiality (session-specific data and artifacts)
- Integrity (trustworthiness and correctness of proxied data and generated artifacts)
- Availability (uptime and service)

### Snap

The officially supported installation method for the Fetch Service is through the snap
maintained by the development team. Security vulnerabilities identified in the snap's
own configuration are considered issues on the Fetch Service itself.

## Supported versions

The Fetch Service is still in development and has no long-term support releases. As such,
only the latest released version is considered supported.

## Reporting a vulnerability

To report a security issue, file a [Private Security Report] with a description of the
issue, the steps you took to create the issue, affected versions, and, if known,
mitigations for the issue.

The [Ubuntu Security disclosure and embargo policy] contains more information about
what you can expect when you contact us and what we expect from you.

[Private Security Report]: https://github.com/canonical/fetch-service/security/advisories/new
[Ubuntu Security disclosure and embargo policy]: https://ubuntu.com/security/disclosure-policy
