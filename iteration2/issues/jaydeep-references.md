## Task: Implement NWFilter References and Parameters

Parent User Story: #10
Developer: Jaydeep Reddy Chintham
GitHub: @Jaydeepreddy1208
Scrum Master: Jaydeep Reddy Chintham

## Objective

Extend libvirt_nwfilter to support ordered filter
references and parameters, allowing users to reuse
existing libvirt network-filter definitions.

## Implementation Tasks

- Review the existing NWFilter entry model.
- Add filter-reference schema support.
- Add reference parameter support.
- Extend XML conversion for filter references.
- Extend Read/Refresh to retrieve reference data.
- Preserve ordered entries in Terraform state.
- Add validation for invalid references.
- Implement unit and acceptance tests.
- Verify compatibility with existing ICMP rules.

## Acceptance Criteria

- Terraform accepts valid filter references.
- References and parameters generate correct libvirt XML.
- Read/Refresh maintains consistent Terraform state.
- Invalid configurations produce useful diagnostics.
- Existing ICMP functionality continues working.
- Unit and acceptance tests pass.
- Changes are peer-reviewed before merging.

## Development Workflow

Feature branch: feature/i2-filter-references
Integration branch: sprint/iteration2

Create a pull request after implementation and testing.

## Scrum Master Responsibilities

- Coordinate regular Scrum meetings.
- Record meeting dates, attendance, and blockers.
- Monitor GitHub issue progress.
- Coordinate sprint review and retrospective.
