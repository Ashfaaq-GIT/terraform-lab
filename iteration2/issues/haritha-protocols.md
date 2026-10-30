## Task: Implement TCP/UDP and IPv4/MAC Support

Parent User Story: #10
Developer: Haritha Reddy Medikonda
GitHub: @harimedik07-coder

## Objective

Extend libvirt_nwfilter with TCP, UDP, IPv4,
and MAC matching capabilities so Terraform users
can define more advanced network-filter policies.

## Implementation Tasks

- Review existing ICMP schema and conversion.
- Add TCP rule support.
- Add UDP rule support.
- Support TCP/UDP source and destination ports.
- Add IPv4 address matching criteria.
- Add MAC address matching criteria.
- Extend the NWFilter entry model.
- Extend libvirt XML generation.
- Validate protocol values, addresses, and port ranges.
- Add unit and negative tests.
- Add real libvirt acceptance tests.
- Verify existing ICMP functionality still works.

## Acceptance Criteria

- Terraform accepts valid TCP and UDP rules.
- Source and destination ports are supported.
- IPv4 and MAC matching criteria work.
- Generated libvirt XML matches the configuration.
- Invalid ports and addresses produce useful errors.
- Existing ICMP tests continue passing.
- New unit and acceptance tests pass.
- Changes are peer-reviewed before merging.

## Development Workflow

Feature branch: feature/i2-protocols
Integration branch: sprint/iteration2

Create a pull request after implementation
and successful testing.
