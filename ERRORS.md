# Errors and Troubleshooting

This file documents errors encountered while building the Terraform/libvirt lab.

The main README contains only the working setup. Troubleshooting history is kept here separately.

## Libvirt Socket

The default provider URI initially looked for the traditional libvirt socket.

The Arch installation uses modular libvirt:

```text
/run/libvirt/virtqemud-sock
```

Working provider URI:

```hcl
provider "libvirt" {
  uri = "qemu:///system?socket=/run/libvirt/virtqemud-sock"
}
```

## Libvirt Modular Services

Required services included:

```bash
sudo systemctl enable --now virtqemud.socket
sudo systemctl enable --now virtnetworkd.socket
sudo systemctl enable --now virtstoraged.socket
sudo systemctl enable --now virtnodedevd.socket
```

## Libvirt Authorization

Error:

```text
authentication failed: access denied by policy
```

Fix:

```bash
sudo usermod -aG libvirt "$USER"
```

Then log out and log back in.

Do not run Terraform with `sudo`.

## Domain OS Type

Error:

```text
XML error: an os <type> must be specified
```

Fix:

```hcl
os = {
  type         = "hvm"
  type_arch    = "x86_64"
  type_machine = "pc"
}
```

## SPICE Graphics Unsupported

Error:

```text
spice graphics are not supported with this QEMU
```

SPICE graphics were removed because they are not required for SSH-based VM management.

## VM Timed Out Waiting for DHCP

Error:

```text
Failed to Wait for IP Address
timeout waiting for IP address after 300 seconds
```

Initially this appeared to be a networking problem.

The actual first problem was that Debian was not booting.

## QCOW2 Interpreted as Raw

Generated domain XML initially showed:

```xml
<driver name='qemu' type='raw'/>
```

for the QCOW2 root disk.

Disk statistics showed no reads:

```text
vda rd_req 0
vda rd_bytes 0
```

Fix:

```hcl
driver = {
  name = "qemu"
  type = "qcow2"
}
```

After the fix, Debian booted successfully and disk reads increased.

## Guest Interface Name

The reference project used:

```yaml
ens3:
  dhcp4: true
```

The Debian 13 VM created by this setup actually used:

```text
enp0s2
```

Fix:

```yaml
version: 2
ethernets:
  primary:
    match:
      name: "en*"
    dhcp4: true
```

After restarting with the corrected cloud-init network configuration, the VM received a DHCP lease.

## Successful DHCP Result

Example:

```text
192.168.122.243/24
```

Inside the VM, the address was shown as dynamic and the route was marked `proto dhcp`.

The address is DHCP-assigned, not statically configured.

## VM Restart After Replacing Disk or Cloud-init

Replacing Terraform volume resources does not necessarily make an already-running QEMU process reopen those files.

After replacing the VM disk or cloud-init ISO, the VM was restarted with:

```bash
virsh -c qemu:///system destroy vm1
virsh -c qemu:///system start vm1
```

## tcpdump Missing

Error:

```text
sudo: tcpdump: command not found
```

Fix:

```bash
sudo pacman -S tcpdump
```

## Active virsh Console

Error:

```text
Active console session exists for this domain
```

Fix:

```bash
virsh -c qemu:///system console vm1 --force
```

## Git Repository Missing

Error:

```text
fatal: not a git repository
```

Fix:

```bash
cd ~/terraform-lab
git init
git branch -m main
```

## Git Pager Missing

Error:

```text
error: cannot run less
```

Use:

```bash
git --no-pager diff --cached
```

or install:

```bash
sudo pacman -S less
```
