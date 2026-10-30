## Task: Implement Safe NWFilter Update Lifecycle

Parent User Story: #10
Developer: Ashfaaq Ahamed Kapatrala
GitHub: @Ashfaaq-GIT

## Objective

Implement safe Update behavior for libvirt_nwfilter
so Terraform can modify an existing network filter
without requiring virtual machine recreation.

## Implementation Tasks

- Review existing NWFilterResource.Update().
- Review Terraform replacement behavior.
- Implement safe network-filter redefinition.
- Preserve Terraform resource identity where supported.
- Integrate Update with Read/Refresh.
- Handle libvirt errors and Terraform diagnostics.
- Verify compatibility with existing ICMP functionality.
- Add unit and acceptance tests.
- Help integrate the Iteration 2 functionality.

## Acceptance Criteria

- Supported filter changes can be applied through Terraform.
- Existing virtual machines are not recreated.
- Updated libvirt XML matches the desired configuration.
- Terraform Read/Refresh produces consistent state.
- Unchanged configuration produces a clean plan.
- Unit and acceptance tests pass.
- Changes are peer-reviewed before merging.

## Development Workflow

Feature branch: feature/i2-nwfilter-update
Integration branch: sprint/iteration2

Create a pull request for review after testing.
