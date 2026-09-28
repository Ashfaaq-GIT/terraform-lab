# Iteration 1 - Basic `libvirt_nwfilter` Lifecycle and ICMP Support

## Project Overview

This project extends the open-source
`terraform-provider-libvirt` Terraform provider with a new first-class
`libvirt_nwfilter` resource.

The provider already allows Terraform users to manage major libvirt
infrastructure resources such as:

- Virtual machines
- Networks
- Storage pools
- Storage volumes
- Cloud-init configuration

However, libvirt network-filter definitions were not managed as their own
first-class Terraform resource.

Before this project, a virtual-machine interface could reference a filter
that already existed in libvirt, but Terraform could not fully manage the
network-filter definition itself.

This created a split workflow:

```text
Terraform
   |
   +--> Virtual Machines
   +--> Networks
   +--> Storage
   +--> Volumes

Network security policy
   |
   +--> separate XML / virsh workflow
```

The goal of this project is to bring libvirt network-filter policy into the
same Terraform workflow.

The final project introduces:

```text
Terraform HCL
     |
     v
terraform-provider-libvirt
     |
     v
libvirt_nwfilter
     |
     v
libvirt network-filter XML
     |
     v
libvirt
     |
     v
QEMU/KVM virtual-machine networking
```

---

# Iteration 1 Objective

Iteration 1 establishes the first usable increment of the
`libvirt_nwfilter` resource.

The Iteration 1 deliverable includes:

- Basic `libvirt_nwfilter` resource
- Create lifecycle
- Read lifecycle
- Refresh behavior
- Delete lifecycle
- Filter name
- Filter chain
- Filter priority
- Ordered rule entries
- ICMP support
- Unit testing
- Acceptance testing against real libvirt
- Live Terraform demonstration
- Reusable demo configuration
- Reusable presentation scripts

Iteration 1 intentionally does **not** implement the complete final project.

Later protocol support, updates, import, complete drift repair, additional
validation, and the final two-VM policy verification remain scheduled for
later iterations.

---

# Team

Team 5

- Ashfaaq Ahamed Kapatrala
- Jaydeep Reddy Chintham
- Haritha Reddy Medikonda

Implementation repository:

```text
https://github.com/Ashfaaq-GIT/terraform-lab
```

Project extended:

```text
https://github.com/dmacvicar/terraform-provider-libvirt
```

Project baseline:

```text
terraform-provider-libvirt v0.9.8
```

Primary technologies:

- Go
- Terraform
- Terraform Plugin Framework
- terraform-plugin-testing
- go-libvirt
- libvirtxml
- libvirt
- QEMU/KVM
- virsh
- cloud-init
- Git
- GitHub

---

# What Existed Before Iteration 1

The upstream provider already manages normal virtualization infrastructure.

Examples include:

```text
libvirt_domain
libvirt_network
libvirt_pool
libvirt_volume
libvirt_cloudinit_disk
```

Our test environment already demonstrated that Terraform could provision a
real Debian virtual machine through libvirt and QEMU/KVM.

The baseline VM environment includes:

```text
Terraform
    |
    v
terraform-provider-libvirt
    |
    v
libvirt
    |
    v
QEMU/KVM
    |
    v
Debian VM
    |
    v
libvirt default network
    |
    v
DHCP IP
    |
    v
SSH
```

What was missing was lifecycle management for the **network-filter
definition itself**.

That missing functionality is the focus of this project.

---

# What We Added in Iteration 1

Iteration 1 adds the initial implementation of:

```hcl
resource "libvirt_nwfilter" "example" {
    ...
}
```

The new resource allows Terraform to define a network filter directly
instead of requiring the filter definition to be created manually outside
Terraform.

The Iteration 1 implementation supports a basic ICMP rule model and the
resource lifecycle necessary to create, read, refresh, and delete the
filter.

---

# Example Iteration 1 Configuration

The reusable demonstration uses a configuration similar to:

```hcl
resource "libvirt_nwfilter" "icmp_demo" {
  name     = "terraform-iteration1-icmp"
  chain    = "root"
  priority = 500

  entries = [
    {
      rule = {
        action    = "accept"
        direction = "in"
        priority  = 100

        icmp = {
          type = 8
          code = 0
        }
      }
    }
  ]
}
```

This configuration describes:

```text
Filter name:
terraform-iteration1-icmp

Chain:
root

Filter priority:
500

Rule action:
accept

Rule direction:
in

Rule priority:
100

Protocol:
ICMP

ICMP type:
8

ICMP code:
0
```

---

# Current Iteration 1 Capability Matrix

| Capability | Status |
|---|---|
| Register `libvirt_nwfilter` with provider | Implemented |
| Filter `name` | Implemented |
| Filter `chain` | Implemented |
| Filter `priority` | Implemented |
| Ordered `entries` list | Implemented |
| Rule entry | Implemented |
| Rule `action` | Implemented |
| Rule `direction` | Implemented |
| Rule `priority` | Implemented |
| ICMP protocol block | Implemented |
| ICMP `type` | Implemented |
| ICMP `code` | Implemented |
| Create lifecycle | Implemented |
| Read lifecycle | Implemented |
| Refresh behavior | Implemented |
| Delete lifecycle | Implemented |
| Missing-filter detection | Implemented |
| ICMP XML generation | Implemented |
| ICMP type range check | Implemented |
| ICMP code range check | Implemented |
| Unit tests | Implemented |
| Acceptance test | Implemented |
| Real libvirt verification | Implemented |
| Live Terraform demo | Implemented |
| Reusable demo scripts | Implemented |

---

# What Is Not Implemented Yet

Iteration 1 is only the first increment.

The following features are deliberately deferred.

| Capability | Planned Stage |
|---|---|
| TCP rules | Iteration 2 |
| UDP rules | Iteration 2 |
| IPv4 match criteria | Iteration 2 |
| MAC match criteria | Iteration 2 |
| Filter references | Iteration 2 |
| Filter-reference parameters | Iteration 2 |
| Safe update behavior | Iteration 2 |
| Expanded validation | Iteration 2 |
| Negative configuration tests | Iteration 2 |
| Live policy-change demonstration | Iteration 2 |
| Import existing nwfilters | Iteration 3 |
| Full drift detection and repair | Iteration 3 |
| ARP support | Iteration 3 |
| IPv6 support | Iteration 3 |
| Full semantic validation | Iteration 3 |
| Full regression testing | Iteration 3 |
| Ansible-verified two-VM demonstration | Iteration 3 |
| Generated final documentation | Iteration 3 |
| Final examples | Iteration 3 |

This distinction is important because the final project requirements are
larger than the Iteration 1 increment.

---

# Iteration 1 Resource Architecture

The resource follows the existing provider architecture.

```text
Terraform HCL
      |
      v
Terraform Plugin Framework
      |
      v
libvirt_nwfilter Schema
      |
      v
Terraform Resource Model
      |
      v
Entry Conversion
      |
      v
libvirtxml.NWFilter
      |
      v
XML
      |
      v
go-libvirt
      |
      v
libvirt nwfilter API
      |
      v
QEMU/KVM networking
```

The main layers are described below.

## Terraform Schema

The Terraform Plugin Framework exposes the resource to Terraform.

The schema describes fields such as:

```text
id
name
chain
priority
entries
```

For Iteration 1, an entry contains a rule and the rule contains an ICMP
block.

---

# Ordered Entry Model

The final project architecture is designed around ordered network-filter
entries.

Conceptually:

```text
Network Filter
    |
    +--> Entry 1
    |
    +--> Entry 2
    |
    +--> Entry 3
```

Each entry in the final design can eventually represent:

```text
Rule
or
Filter Reference
```

Iteration 1 establishes the ordered `entries` structure but implements
**rule entries only**.

Filter references are reserved for Iteration 2.

---

# ICMP Rule Model

For Iteration 1, the supported rule structure is:

```text
entry
  |
  +--> rule
         |
         +--> action
         +--> direction
         +--> priority
         |
         +--> icmp
                |
                +--> type
                +--> code
```

Example:

```hcl
entries = [
  {
    rule = {
      action    = "accept"
      direction = "in"
      priority  = 100

      icmp = {
        type = 8
        code = 0
      }
    }
  }
]
```

---

# Terraform to libvirt Conversion

The Iteration 1 conversion path is:

```text
Terraform configuration
        |
        v
NWFilterResourceModel
        |
        v
nwFilterEntriesFromModel()
        |
        v
buildICMPNWFilterEntry()
        |
        v
libvirtxml.NWFilterEntry
        |
        v
libvirtxml.NWFilterRule
        |
        v
libvirtxml.NWFilterRuleICMP
        |
        v
NWFilter XML
```

ICMP type and code values are represented using
`libvirtxml.NWFilterField`.

The resulting structure is marshalled to XML before being sent to libvirt.

---

# Create Lifecycle

The Create operation performs the following flow:

```text
Terraform Apply
     |
     v
Read planned resource model
     |
     v
Build NWFilter structure
     |
     v
Convert entries
     |
     v
Build ICMP rule
     |
     v
Marshal libvirt XML
     |
     v
NwfilterDefineXML
     |
     v
libvirt creates nwfilter
     |
     v
Store UUID in Terraform state
```

Terraform therefore creates a real libvirt network-filter resource rather
than only storing configuration locally.

---

# Read / Refresh Lifecycle

The Read operation allows Terraform to refresh the resource from the real
libvirt environment.

The flow is:

```text
Terraform Refresh / Plan
        |
        v
Read current Terraform state
        |
        v
Lookup nwfilter by name
        |
        +----------------------------+
        |                            |
        v                            v
Filter exists                 Filter missing
        |                            |
        v                            v
Get live XML              Remove resource from state
        |
        v
Unmarshal XML
        |
        v
Refresh resource identity
        |
        v
Write refreshed Terraform state
```

This makes Terraform aware when the network filter exists or has been
deleted outside Terraform.

A normal unchanged resource produces a clean Terraform plan.

During the live demonstration, the second plan returned:

```text
No changes. Your infrastructure matches the configuration.
```

---

# Delete Lifecycle

Delete performs:

```text
terraform destroy
      |
      v
Lookup filter by name
      |
      v
NwfilterUndefine
      |
      v
libvirt removes filter
```

After destruction the filter can be checked directly with:

```bash
virsh -c qemu:///system nwfilter-list
```

Our live demonstration confirmed that the filter was removed.

---

# Update Behavior in Iteration 1

Iteration 1 does **not** implement true in-place update behavior.

The configurable resource fields currently use replacement semantics where
appropriate.

This means that changing those fields may cause Terraform to replace the
resource rather than modifying the existing libvirt filter in place.

Safe update support is planned for Iteration 2.

---

# ICMP Validation

Iteration 1 performs basic ICMP type and code checks.

Supported range:

```text
0 through 255
```

The implementation rejects values such as:

```text
type = -1
type = 256

code = -1
code = 256
```

These checks are currently part of the Iteration 1 conversion logic.

More complete semantic and plan-time validation is scheduled for later
iterations.

---

# Live libvirt XML

Terraform configuration such as:

```hcl
icmp = {
  type = 8
  code = 0
}
```

produced real libvirt XML similar to:

```xml
<filter name='terraform-iteration1-icmp'
        chain='root'
        priority='500'>

  <rule action='accept'
        direction='in'
        priority='100'>

    <icmp type='0x8' code='0x0'/>

  </rule>

</filter>
```

The hexadecimal representation shown by libvirt corresponds to:

```text
0x8 = decimal 8
0x0 = decimal 0
```

This demonstrates the complete conversion:

```text
Terraform HCL
     |
     v
Go model
     |
     v
libvirtxml
     |
     v
libvirt XML
```

---

# Known Iteration 1 Limitations

Iteration 1 is intentionally not the final resource implementation.

## ICMP Only

The Iteration 1 nested protocol model currently supports ICMP.

It does not yet support:

```text
TCP
UDP
IPv4 criteria
MAC criteria
ARP
IPv6
```

---

## Filter References Are Not Yet Supported

The long-term architecture supports entries that are either:

```text
rule
```

or:

```text
filter reference
```

Iteration 1 currently implements only rule entries.

Filter references and parameters are planned for Iteration 2.

---

## No In-Place Update

Iteration 1 focuses on:

```text
Create
Read
Refresh
Delete
```

True safe update behavior is planned for Iteration 2.

---

## Limited Semantic Validation

Iteration 1 verifies the basic ICMP numeric range.

It does not yet implement complete validation for:

- Conflicting protocol fields
- Network-address semantics
- Port ranges
- Filter-reference combinations
- Protocol-specific combinations

Expanded validation is scheduled for later work.

---

## Full Rule Drift Repair Is Not Yet Implemented

Iteration 1 Read can:

- Locate the live filter
- Retrieve its XML
- Refresh resource identity
- Preserve configured state
- Detect when the filter no longer exists

However, complete comparison and repair of externally modified rule-entry
content is not part of Iteration 1.

Full drift detection and repair is planned for Iteration 3.

---

## Import Is Not Implemented

Iteration 1 cannot yet import an existing manually-created libvirt nwfilter
into Terraform state.

Import support is planned for Iteration 3.

---

## Two-VM Policy Enforcement Is Not an Iteration 1 Requirement

Iteration 1 demonstrates creation and lifecycle management of the real
libvirt network filter.

The final two-VM test where policy changes are verified from inside the
guests is planned for Iteration 3.

That final demonstration will verify behavior such as ICMP allow/block
while SSH remains reachable.

---

# Main Source Files

The primary resource implementation is:

```text
terraform-provider-libvirt/
└── internal/provider/
    └── nwfilter_resource.go
```

This file contains the Iteration 1 resource schema and lifecycle logic.

---

# Provider Registration

The resource is registered with the provider in:

```text
terraform-provider-libvirt/
└── internal/provider/
    └── provider.go
```

This registration allows Terraform to recognize:

```text
libvirt_nwfilter
```

as a resource type.

---

# Unit Tests

Unit tests are located in:

```text
terraform-provider-libvirt/
└── internal/provider/
    └── nwfilter_resource_test.go
```

The unit tests verify:

- Rule creation
- Action
- Direction
- Priority
- ICMP type
- ICMP code
- XML marshal
- XML unmarshal
- XML round trip
- Negative ICMP type
- ICMP type greater than 255
- Negative ICMP code
- ICMP code greater than 255

Main tests include:

```text
TestBuildICMPNWFilterEntry
TestBuildICMPNWFilterEntryRejectsInvalidValues
```

Run them as part of the provider test suite:

```bash
cd ~/terraform-lab/terraform-provider-libvirt
make test
```

The Iteration 1 tests passed successfully.

---

# Provider Build Verification

The provider is built using:

```bash
cd ~/terraform-lab/terraform-provider-libvirt
make build
```

The Iteration 1 implementation builds successfully with the rest of the
provider.

This is important because the new resource must not break the existing
provider build.

---

# Acceptance Testing

Acceptance testing is located in:

```text
terraform-provider-libvirt/
└── internal/provider/
    └── nwfilter_resource_acc_test.go
```

The Iteration 1 acceptance test is:

```text
TestAccNWFilterResource_ICMP
```

Run it with:

```bash
cd ~/terraform-lab/terraform-provider-libvirt

TF_ACC=1 \
LIBVIRT_TEST_URI='qemu:///system' \
go test ./internal/provider \
  -run '^TestAccNWFilterResource_ICMP$' \
  -v \
  -count=1
```

Successful result:

```text
=== RUN   TestAccNWFilterResource_ICMP
--- PASS: TestAccNWFilterResource_ICMP
PASS
```

This is a real integration test against the system libvirt daemon.

It is not only a mocked unit test.

---

# What the Acceptance Test Verifies

The acceptance test verifies the following lifecycle:

```text
Terraform creates resource
        |
        v
libvirt filter exists
        |
        v
Read live XML
        |
        v
Verify one entry
        |
        v
Verify action = accept
        |
        v
Verify direction = in
        |
        v
Verify ICMP type = 8
        |
        v
Verify ICMP code = 0
        |
        v
Terraform destroys resource
        |
        v
Verify filter is gone
```

This gives Iteration 1 real-infrastructure verification.

---

# Full Test Suite

The normal provider test suite is executed with:

```bash
cd ~/terraform-lab/terraform-provider-libvirt
make test
```

The new ICMP unit tests run as part of this suite.

Acceptance tests are skipped during the normal suite unless `TF_ACC=1` is
set.

The dedicated ICMP acceptance test was therefore also executed separately
with `TF_ACC=1`.

---

# Baseline VM Infrastructure

The repository also contains:

```text
terraform-vms/
```

This environment demonstrates the underlying Terraform/libvirt/QEMU
infrastructure before demonstrating the custom network-filter resource.

The baseline environment provisions:

```text
libvirt storage pool
        |
        v
Debian base image
        |
        v
VM disk
        |
        v
cloud-init
        |
        v
QEMU/KVM domain
        |
        v
default libvirt network
        |
        v
DHCP address
        |
        v
SSH
```

This baseline is useful during the presentation because it proves the
virtualization environment is functioning before showing the new resource.

---

# Baseline VM SSH Verification

Cloud-init configures the guest user:

```text
vmadmin
```

and installs the host SSH public key.

Terraform also exposes the VM IP address as:

```text
vm1_ip
```

The VM can therefore be verified using SSH.

Example guest verification commands:

```bash
hostname
whoami
ip addr
cat /etc/os-release
```

Expected concepts demonstrated:

```text
hostname -> vm1
user     -> vmadmin
OS       -> Debian
network  -> libvirt DHCP address
```

---

# Why the VM Demo and Iteration 1 Demo Are Separate

The presentation deliberately separates the existing infrastructure from
the new provider functionality.

## Stage 1 - Baseline

```text
Terraform
    |
    v
libvirt
    |
    v
QEMU/KVM
    |
    v
Debian VM
    |
    v
DHCP
    |
    v
SSH
```

This proves the virtualization test environment works.

## Stage 2 - Iteration 1 Extension

```text
Terraform HCL
      |
      v
custom terraform-provider-libvirt
      |
      v
libvirt_nwfilter
      |
      v
ICMP XML
      |
      v
libvirt nwfilter API
```

This makes it clear which functionality existed before the project and
which functionality was added during Iteration 1.

---

# Reusable Live Demo

The Iteration 1 Terraform demo is stored in:

```text
iteration1-live-demo/
```

Contents include:

```text
iteration1-live-demo/
├── .gitignore
├── README.md
└── main.tf
```

The directory contains a minimal configuration focused only on
`libvirt_nwfilter`.

This prevents the Iteration 1 demonstration from being mixed with the
larger VM infrastructure configuration.

---

# Presentation Scripts

Reusable scripts are stored in:

```text
scripts/
├── demo-vm.sh
└── demo-iteration1.sh
```

---

# VM Presentation Script

The baseline VM demonstration is controlled by:

```text
scripts/demo-vm.sh
```

Available commands:

```bash
./scripts/demo-vm.sh apply
./scripts/demo-vm.sh status
./scripts/demo-vm.sh ssh
./scripts/demo-vm.sh destroy
```

## `apply`

Demonstrates:

```text
Terraform configuration
        |
        v
terraform init
        |
        v
terraform validate
        |
        v
terraform plan
        |
        v
terraform apply
        |
        v
Running VM
```

## `status`

Shows:

- Terraform state
- Terraform outputs
- libvirt domains
- libvirt networks
- DHCP leases

## `ssh`

Retrieves the Terraform `vm1_ip` output and connects to:

```text
vmadmin@<vm1_ip>
```

## `destroy`

Destroys the baseline VM infrastructure and verifies cleanup.

---

# Iteration 1 Presentation Script

The nwfilter demonstration is controlled by:

```text
scripts/demo-iteration1.sh
```

Available commands:

```bash
./scripts/demo-iteration1.sh setup
./scripts/demo-iteration1.sh apply
./scripts/demo-iteration1.sh verify
./scripts/demo-iteration1.sh refresh
./scripts/demo-iteration1.sh status
./scripts/demo-iteration1.sh destroy
./scripts/demo-iteration1.sh all
```

---

# Iteration 1 Demo - Setup

Run:

```bash
./scripts/demo-iteration1.sh setup
```

This:

1. Builds the modified provider.
2. Stores the provider binary in `.provider-dev/`.
3. Creates the local Terraform development override.
4. Configures Terraform to use the locally-built provider.

The local binary and development configuration are not committed because
they are machine-specific runtime artifacts.

---

# Iteration 1 Demo - Apply

Run:

```bash
./scripts/demo-iteration1.sh apply
```

This demonstrates:

```text
Terraform configuration
      |
      v
Validation
      |
      v
Plan
      |
      v
Create libvirt_nwfilter
```

Expected plan:

```text
Plan: 1 to add, 0 to change, 0 to destroy.
```

Expected apply result:

```text
Apply complete! Resources: 1 added, 0 changed, 0 destroyed.
```

---

# Iteration 1 Demo - Verify

Run:

```bash
./scripts/demo-iteration1.sh verify
```

This performs:

```bash
virsh -c qemu:///system nwfilter-list
```

and:

```bash
virsh -c qemu:///system \
  nwfilter-dumpxml terraform-iteration1-icmp
```

This verifies that Terraform created a real libvirt filter.

---

# Iteration 1 Demo - Refresh

Run:

```bash
./scripts/demo-iteration1.sh refresh
```

Terraform refreshes the resource and produces:

```text
No changes. Your infrastructure matches the configuration.
```

This demonstrates the Iteration 1 Read / Refresh lifecycle.

---

# Iteration 1 Demo - Destroy

Run:

```bash
./scripts/demo-iteration1.sh destroy
```

This destroys the filter and verifies removal.

Expected output includes:

```text
Destroy complete! Resources: 1 destroyed.
```

and:

```text
filter successfully removed
```

---

# Complete Presentation Flow

The recommended presentation sequence is:

```text
PART 1 - BASELINE INFRASTRUCTURE

./scripts/demo-vm.sh apply

./scripts/demo-vm.sh status

./scripts/demo-vm.sh ssh

Inside VM:
    hostname
    whoami
    ip addr
    cat /etc/os-release
    exit

./scripts/demo-vm.sh destroy


PART 2 - ITERATION 1

./scripts/demo-iteration1.sh setup

./scripts/demo-iteration1.sh apply

./scripts/demo-iteration1.sh verify

./scripts/demo-iteration1.sh refresh

./scripts/demo-iteration1.sh destroy
```

---

# Team Contributions

## Ashfaaq Ahamed Kapatrala

Iteration 1 responsibility:

```text
Create / Delete lifecycle
```

Work included:

- Initial `libvirt_nwfilter` resource
- Resource registration
- Resource schema foundation
- Create implementation
- XML generation
- `NwfilterDefineXML`
- Terraform UUID state
- Delete implementation
- `NwfilterUndefine`

Primary PR:

```text
PR #1 - Add nwfilter create and delete lifecycle
```

---

## Jaydeep Reddy Chintham

Iteration 1 responsibility:

```text
Read / Refresh lifecycle
```

Work included:

- Lookup network filter by name
- Handle missing filters
- Retrieve live XML
- Unmarshal live filter
- Refresh ID
- Refresh name
- Refresh configured top-level values
- Remove missing resource from Terraform state

Primary PR:

```text
PR #5 - Implement nwfilter read and refresh lifecycle
```

---

## Haritha Reddy Medikonda

Iteration 1 responsibility:

```text
ICMP support and testing
```

Work included:

- Ordered `entries` schema
- Rule structure
- ICMP nested block
- ICMP type conversion
- ICMP code conversion
- ICMP validation
- XML round-trip tests
- Invalid-value tests
- Acceptance test
- Real libvirt verification

Primary PR:

```text
PR #6 - Add ICMP nwfilter support and tests
```

---

# Supporting Pull Requests

Additional documentation/demo work includes:

```text
PR #7 - Add Iteration 1 nwfilter live demo
PR #8 - Add VM and Iteration 1 demo scripts
```

These supporting changes make the implementation reproducible during
demonstration and grading.

---

# Iteration 1 Issues

Primary Iteration 1 work was tracked through:

```text
Issue #2
Iteration 1: Basic libvirt_nwfilter lifecycle and ICMP support

Issue #3
Person 2: Implement nwfilter Read and Refresh

Issue #4
Person 3: Add ICMP support and Iteration 1 tests
```

The development work for these Iteration 1 issues was completed.

---

# Testing Results

| Test / Verification | Result |
|---|---|
| Provider resource registration | PASS |
| Go compilation | PASS |
| Create lifecycle | PASS |
| Read lifecycle | PASS |
| Refresh lifecycle | PASS |
| Delete lifecycle | PASS |
| ICMP rule construction | PASS |
| ICMP XML round trip | PASS |
| Negative ICMP type test | PASS |
| ICMP type >255 test | PASS |
| Negative ICMP code test | PASS |
| ICMP code >255 test | PASS |
| Full provider unit suite | PASS |
| Provider regression suite | PASS |
| Provider build | PASS |
| Real libvirt acceptance test | PASS |
| Terraform live plan | PASS |
| Terraform live apply | PASS |
| `virsh nwfilter-list` verification | PASS |
| Live XML verification | PASS |
| Terraform no-change refresh | PASS |
| Terraform destroy | PASS |
| libvirt cleanup verification | PASS |

---

# Iteration 1 Definition of Done

Iteration 1 is considered complete because the planned Iteration 1
increment was demonstrated end-to-end.

The following were verified:

- Terraform recognizes `libvirt_nwfilter`.
- Terraform can declare an ICMP network filter.
- Terraform can plan creation of the filter.
- Terraform can create the filter in real libvirt.
- The created resource receives a libvirt UUID.
- `virsh` can see the created filter.
- The live XML contains the configured ICMP rule.
- ICMP type and code survive Terraform-to-XML conversion.
- Terraform can refresh the resource.
- An unchanged resource produces a clean plan.
- Terraform can delete the resource.
- libvirt confirms the resource is removed.
- Unit tests pass.
- Acceptance tests pass.
- Existing regression tests pass.
- The provider builds successfully.
- The demonstration can be reproduced from scripts stored in the
  repository.

---

# Iteration 2 Planned Work

Iteration 2 expands the rule model beyond basic ICMP support.

Planned functionality includes:

- TCP
- UDP
- IPv4 criteria
- MAC criteria
- Port-related fields
- Filter references
- Filter-reference parameters
- Safe update behavior
- Additional validation
- Negative tests
- Live policy-change demonstration

The goal is to move from the minimum lifecycle implementation to a broader
network-policy model.

---

# Iteration 3 Planned Work

Iteration 3 completes the advanced lifecycle and verification features.

Planned functionality includes:

- Import existing filters
- Full drift detection
- Drift repair
- ARP
- IPv6
- Full semantic validation
- Complete regression testing
- Ansible automation
- Two-VM network-security verification
- ICMP allow/block behavior
- SSH reachability verification
- Final examples
- Generated documentation

---

# Project Roadmap

```text
ITERATION 1
Basic lifecycle
Create
Read
Refresh
Delete
ICMP
Tests
Live demo
        |
        v
ITERATION 2
TCP
UDP
IPv4
MAC
References
Parameters
Updates
Validation
        |
        v
ITERATION 3
Import
Drift repair
ARP
IPv6
Semantic validation
Ansible
Two-VM verification
Final documentation
```

---

# Repository Layout Relevant to Iteration 1

```text
terraform-lab/
|
├── README.md
|
├── iteration1/
│   └── README.md
|
├── iteration1-live-demo/
│   ├── .gitignore
│   ├── README.md
│   └── main.tf
|
├── scripts/
│   ├── demo-vm.sh
│   └── demo-iteration1.sh
|
├── terraform-vms/
│   ├── main.tf
│   ├── cloud_init.cfg
│   ├── network_config.cfg
│   └── templates/
|
├── terraform-provider-libvirt/
│   └── internal/provider/
│       ├── provider.go
│       ├── nwfilter_resource.go
│       ├── nwfilter_resource_test.go
│       └── nwfilter_resource_acc_test.go
|
├── terraform-libvirt-test/
|
└── terraform-basic/
```

---

# Documentation Structure

The repository intentionally contains different README files for different
purposes.

## Root `README.md`

Purpose:

```text
Overall project and Terraform/libvirt lab documentation
```

## `iteration1/README.md`

Purpose:

```text
Explain WHAT was implemented in Iteration 1,
what works,
how it works,
what was tested,
and what is still not implemented.
```

## `iteration1-live-demo/README.md`

Purpose:

```text
Explain HOW to reproduce the Iteration 1 live Terraform demonstration.
```

This separation keeps the implementation summary and demonstration
instructions clear.

---

# Iteration 1 Final Status

```text
Resource registration      COMPLETE
Create lifecycle           COMPLETE
Read lifecycle             COMPLETE
Refresh lifecycle          COMPLETE
Delete lifecycle           COMPLETE
Ordered entries foundation COMPLETE
ICMP support               COMPLETE
ICMP validation            COMPLETE
Unit testing               COMPLETE
Acceptance testing         COMPLETE
Provider regression tests  COMPLETE
Provider build             COMPLETE
Live libvirt verification  COMPLETE
Terraform refresh demo     COMPLETE
Terraform destroy demo     COMPLETE
Reusable demo              COMPLETE
Presentation automation    COMPLETE
```

## Iteration 1 Status

**COMPLETE**

The project now has a functioning first-class
`libvirt_nwfilter` Terraform resource capable of managing the basic
network-filter lifecycle and creating verified ICMP rules on real libvirt
infrastructure.

Later iterations will extend this foundation rather than replacing it.
