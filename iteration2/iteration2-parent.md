## User Story

As a Terraform user, I want to manage advanced libvirt
network filters with TCP/UDP, IPv4/MAC criteria, filter
references, and safe updates so that network security
policies can be managed through Terraform without
recreating virtual machines.

## Iteration 2 Goals

- Implement TCP and UDP rule support.
- Add IPv4 and MAC matching criteria.
- Support filter references and parameters.
- Implement safe network-filter Update lifecycle.
- Extend Read/Refresh for new attributes.
- Add semantic validation and negative tests.
- Perform unit and integration testing.
- Demonstrate live policy changes and cleanup.

## Team Responsibilities

Ashfaaq:
- Safe Update implementation
- Integration and live demonstration

Jaydeep (Scrum Master):
- Filter references and parameters
- Read/Refresh integration
- Scrum coordination

Haritha:
- TCP/UDP, IPv4/MAC implementation
- Protocol validation and testing

## Acceptance Criteria

- Supported rules generate correct libvirt XML.
- Invalid configurations produce useful errors.
- Unit and acceptance tests pass.
- Existing ICMP functionality remains working.
- Filter updates work without VM recreation.
- Terraform refresh maintains consistent state.
- Live demonstration and cleanup succeed.

## Development Process

Each team member uses a separate feature branch,
commits under their own GitHub identity, opens a PR,
and obtains peer review before merging.
