# Iteration 3 — Comprehensive Network Filter Management

- **Course:** CSCE 5430
- **Team:** Team 5
- **Project:** Terraform Provider for libvirt
- **Sprint branch:** `sprint/iteration3`
**Parent GitHub issue:** #17

## Overview

Iteration 3 extends the `libvirt_nwfilter` Terraform resource with
network-filter import, configuration drift detection and repair,
and ARP/IPv6 protocol support.

The implementation builds on Iteration 2, which added TCP, UDP,
IPv4, MAC, filter references, and safe updates.

## Team Contributions

| Team member | Responsibility | Issue | Pull request |
|---|---|---|---|
| Ashfaaq | Network-filter import | #18 | #21 |
| Jaydeep Reddy Chintham | Drift detection and repair | #19 | #22 |
| Haritha Reddy Medikonda | ARP and IPv6 support | #20 | #23 |

All three feature pull requests were merged into `sprint/iteration3`.
Feature issues #18, #19, and #20 are closed.

Team members used the shared development laptop. Git commits record
the actual contributor as author; Ashfaaq coordinated GitHub pushes
and pull requests. Team coordination was performed offline.

## Architecture

```text
Terraform Configuration (.tf)
          |
          v
Terraform Plugin Framework
          |
          v
libvirt_nwfilter Resource
  - Schema and validation
  - Create / Read / Update / Delete
  - ImportState
          |
          v
Terraform Rule <-> libvirtxml Conversion
          |
          v
libvirt API / NWFilter XML
          |
          v
Host libvirt Network Filter
```

The provider converts Terraform rule blocks into libvirt XML.
During refresh, it reads live XML back into Terraform state.

## Implemented Features

### I3-US1 — Import Existing Filters

- Import a network filter using its UUID.
- Resolve the live filter and decode its rules.
- Reconstruct Terraform resource state.
- Support previously implemented protocol rules and references.

Contributor: Ashfaaq. PR #21, Issue #18.

### I3-US2 — Detect and Repair Drift

- Read the actual filter rules from libvirt.
- Detect configuration differences introduced outside Terraform.
- Produce a nonempty plan when drift occurs.
- Restore the desired rules through Terraform apply.
- Preserve the filter UUID during repair.

Contributor: Jaydeep Reddy Chintham. PR #22, Issue #19.

### I3-US3 — ARP and IPv6 Support

- Configure ARP address, MAC, mask, and opcode criteria.
- Configure IPv6 source/destination addresses and protocol criteria.
- Generate corresponding libvirt XML.
- Decode ARP and IPv6 rules during refresh and import.
- Validate invalid addresses, masks, opcodes, and rule combinations.
- Recognize symbolic ARP opcodes returned by libvirt.

Contributor: Haritha Reddy Medikonda. PR #23, Issue #20.

## Testing

The team executed tests against the provider and a live libvirt
environment.

Verified results include:

- Import unit and acceptance tests: PASS.
- Drift detection and repair tests: PASS.
- ARP XML and validation unit tests: PASS.
- IPv6 XML and validation unit tests: PASS.
- ARP/IPv6 refresh tests: PASS.
- Named ARP opcode regression tests: PASS.
- Live ARP/IPv6 creation, update, and import test: PASS.
- Targeted NWFilter regression suite: PASS.

Example regression command, from `terraform-provider-libvirt/`:

```bash
TF_ACC=1 go test ./internal/provider \
  -run '^(TestAccNWFilterResource_|TestNWFilterARP|TestNWFilterIPv6|TestNWFilterImport|TestNWFilterHaritha|TestNWFilterReferences|TestBuildICMPNWFilterEntry)' \
  -v -count=1 -timeout 10m
```

Acceptance tests require access to a working libvirt environment.

## GitHub Collaboration

- Import: PR #21 merged, Issue #18 closed.
- Drift detection: PR #22 merged, Issue #19 closed.
- ARP/IPv6: PR #23 merged, Issue #20 closed.

The feature changes were merged into the sprint branch rather
than directly into `main`.

## Remaining Integration and Documentation Work

The following activities require separate verification and evidence:

- Integration with the two-VM Terraform environment.
- Ansible connectivity and configuration testing.
- GUI demonstration and screenshots.
- Complete architecture diagrams and setup instructions.
- Final Iteration 3 report and evidence organization.

Do not mark these activities as completed until their tests
or demonstrations have been verified.
