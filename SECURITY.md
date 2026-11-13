# Security Policy

## Reporting a vulnerability

Please report security issues through GitHub's private vulnerability reporting
("Report a vulnerability" on the Security tab of this repository). Do not open
a public issue for a vulnerability.

## Scope notes

mannequin is a test server. It is meant for test suites, not production
traffic: it performs no authentication and no request validation beyond what
its matchers do.

Cassette files record request and response bodies verbatim. Review cassettes
before committing them, prefer synthetic fixtures, and use the scrub hook in
`Record` to censor anything private. Request headers are never recorded.
