# Iteration 3 — Terraform NWFilter Usage Guide

## Prerequisites

- Terraform CLI installed.
- Go toolchain available for building the provider.
- A working libvirt installation with NWFilter support.
- Permission to access `qemu:///system`.
- The Team 5 modified terraform-provider-libvirt build.

IMPORTANT: Use the Team 5 provider build. A published registry
release may not contain the custom `libvirt_nwfilter` resource.

The provider source is located at:

`terraform-provider-libvirt/`

Build and install the development provider using the repository's
build instructions. See `terraform-provider-libvirt/README.md`.

## Example: ARP and IPv6 Network Filter

Create a Terraform configuration such as `main.tf`:

```hcl
terraform {
  required_providers {
    libvirt = {
      source = "dmacvicar/libvirt"
    }
  }
}

provider "libvirt" {
  uri = "qemu:///system"
}

resource "libvirt_nwfilter" "security_policy" {
  name = "team5-iteration3-security"

  entries = [
    {
      rule = {
        action    = "accept"
        direction = "in"

        arp = {
          src_mac = "52:54:00:12:34:56"
          src_ip  = "192.0.2.10"
          opcode  = 2
        }
      }
    },
    {
      rule = {
        action    = "accept"
        direction = "in"

        ipv6 = {
          src_ip      = "2001:db8::10"
          src_ip_mask = "64"
          protocol    = "tcp"
        }
      }
    }
  ]
}

output "filter_uuid" {
  value = libvirt_nwfilter.security_policy.id
}
```

The addresses above are demonstration values. Replace them with
values appropriate to the intended test environment.

## Create and Verify a Filter

From the Terraform configuration directory:

```bash
terraform init
terraform validate
terraform plan
terraform apply
terraform output filter_uuid
```

Inspect the resulting live filter:

```bash
virsh -c qemu:///system nwfilter-list
virsh -c qemu:///system nwfilter-dumpxml team5-iteration3-security
```

Confirm that the XML contains `<arp>` and `<ipv6>` elements.

Do not use these demonstration rules as an unreviewed production
security policy.

## Import an Existing NWFilter

First, obtain the filter UUID:

```bash
virsh -c qemu:///system nwfilter-list
```

Declare a corresponding `libvirt_nwfilter` resource in Terraform
configuration and then import the existing resource by UUID:

```bash
terraform import libvirt_nwfilter.security_policy UUID_FROM_VIRSH
terraform state show libvirt_nwfilter.security_policy
terraform plan
```

Replace `UUID_FROM_VIRSH` with the actual UUID.

The provider resolves the UUID and reads supported rule and
reference entries from the live libvirt XML.

For an empty or incomplete Terraform configuration, the subsequent
plan may propose changes. Review the plan and reconcile the
configuration with the imported filter before applying anything.

## Detect and Repair Configuration Drift

Configuration drift occurs when libvirt's live filter rules differ
from the rules declared in Terraform.

Suggested demonstration workflow:

1. Apply the desired Terraform configuration.
2. Record the filter UUID and live XML.
3. Modify a dedicated test filter outside Terraform.
4. Run `terraform plan` to detect the difference.
5. Review the proposed repair.
6. Run `terraform apply` to restore the declared configuration.
7. Run `terraform plan` again to confirm no further changes.

Only perform out-of-band modifications on an isolated test filter.
Changing an active security policy can affect guest traffic.

The Iteration 3 drift acceptance test verifies detection, repair,
UUID preservation, and a final clean plan.

## Validation

The provider rejects invalid combinations and values, including:

- Multiple protocol blocks in a single rule.
- Invalid ARP MAC or IPv4 addresses.
- Invalid ARP opcodes outside the supported numeric range.
- IPv4 addresses in IPv6-only fields.
- IPv6 prefix masks outside 0–128.
- Unsupported IPv6 protocol identifiers.

Libvirt may return ARP opcodes as symbolic names such as `Request`
and `Reply`. The Iteration 3 decoder converts supported names to
their numeric Terraform representation.

## Tests

From `terraform-provider-libvirt/`, run unit tests:

```bash
go test ./internal/provider -run '^TestNWFilterARP|^TestNWFilterIPv6' -v
```

For the live ARP/IPv6 acceptance test:

```bash
TF_ACC=1 go test ./internal/provider \
  -run '^TestAccNWFilterResource_ARPIPv6$' \
  -v -count=1 -timeout 10m
```

The live test requires accessible libvirt and creates temporary
test resources.

## Clean Up

To remove a Terraform-managed demonstration filter:

```bash
terraform destroy
```

Review the destroy plan before confirming. Never destroy shared
resources or active policies unintentionally.

## Related Documentation

- `iteration3/README.md` — Sprint overview and contributions.
- `iteration1/README.md` — Original NWFilter resource lifecycle.
- `iteration2/` — TCP/UDP, IPv4/MAC, references, and safe updates.
- `terraform-provider-libvirt/README.md` — Provider build instructions.
