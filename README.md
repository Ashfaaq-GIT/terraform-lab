# Terraform Libvirt Lab

A hands-on Terraform, libvirt, QEMU/KVM, cloud-init, and Ansible inventory lab on Arch Linux.



This lab adapts the VM, networking, cloud-init, and inventory approach to:

- Terraform
- dmacvicar/libvirt 0.9.9
- Debian 13
- Arch Linux host
- modular libvirt daemons

## Project Structure

```text
terraform-lab/
├── README.md
├── ERRORS.md
├── .gitignore
├── terraform-basic/
├── terraform-libvirt-test/
└── terraform-vms/
    ├── main.tf
    ├── cloud_init.cfg
    ├── network_config.cfg
    ├── .terraform.lock.hcl
    ├── templates/
    │   └── hosts.yml.tftpl
    ├── ansible/
    │   └── inventory/
    │       └── hosts.ini
    └── images/
        └── debian-13-generic-amd64.qcow2
```

Generated Terraform state, plans, QCOW2 images, `.terraform/`, and generated Ansible inventory files are ignored by Git.

## Terraform Basic

`terraform-basic/` contains the initial Terraform test used to verify Terraform itself before adding libvirt.

Typical commands:

```bash
terraform init
terraform fmt
terraform validate
terraform plan
terraform apply
```

## Libvirt Provider Test

`terraform-libvirt-test/` verifies that Terraform can communicate with the local libvirt installation.

The Arch Linux host uses the modular QEMU socket:

```text
/run/libvirt/virtqemud-sock
```

Provider configuration:

```hcl
provider "libvirt" {
  uri = "qemu:///system?socket=/run/libvirt/virtqemud-sock"
}
```

## VM Configuration

The working VM is configured with:

```text
Name:       vm1
Memory:     2048 MiB
vCPU:       2
Disk:       20 GiB QCOW2 overlay
Base image: Debian 13 generic amd64
Network:    libvirt default NAT network
User:       vmadmin
```

The base image is stored locally under:

```text
terraform-vms/images/debian-13-generic-amd64.qcow2
```

The image itself is not committed to Git.

## QCOW2 Disk

The VM writable disk uses the Debian base image as a backing store.

The domain disk must explicitly use:

```hcl
driver = {
  name = "qemu"
  type = "qcow2"
}
```

## Cloud-init

`cloud_init.cfg` creates the `vmadmin` user.

SSH authentication uses the local host's Ed25519 public key.

Terraform loads it dynamically:

```hcl
ssh_public_key = trimspace(file(pathexpand("~/.ssh/id_ed25519.pub")))
```

SSH password authentication is disabled.

## Networking

The VM uses libvirt's existing `default` NAT network.

Host-side network:

```text
Bridge:   virbr0
Gateway:  192.168.122.1
Network:  192.168.122.0/24
```

The guest receives its address using DHCP.

`network_config.cfg`:

```yaml
version: 2
ethernets:
  primary:
    match:
      name: "en*"
    dhcp4: true
```

On the tested Debian 13 VM, the interface appeared as:

```text
enp0s2
```

## Terraform IP Discovery

Terraform waits for a DHCP lease:

```hcl
wait_for_ip = {
  source  = "lease"
  timeout = 300
}
```

It then reads the interface address through:

```hcl
data "libvirt_domain_interface_addresses" "vm1" {
  domain = libvirt_domain.vm1.name
  source = "lease"
}
```

Display the discovered IP with:

```bash
terraform output vm1_ip
```

## Ansible Inventory

Terraform renders the discovered DHCP address into:

```text
terraform-vms/ansible/inventory/hosts.ini
```

using:

```text
terraform-vms/templates/hosts.yml.tftpl
```

Template:

```ini
[vms]
vm1 ansible_host=${vm1_ip} ansible_user=vmadmin
```

Example generated inventory:

```ini
[vms]
vm1 ansible_host=192.168.122.243 ansible_user=vmadmin
```

The generated inventory is ignored by Git.

## Provision VM

```bash
cd terraform-vms

terraform init
terraform fmt
terraform validate
terraform plan -out=vm1.plan
terraform apply vm1.plan
```

Verify:

```bash
terraform output vm1_ip
cat ansible/inventory/hosts.ini
virsh -c qemu:///system net-dhcp-leases default
```

SSH:

```bash
ssh vmadmin@$(awk -F'ansible_host=' '/vm1/ {split($2,a," "); print a[1]}' ansible/inventory/hosts.ini)
```

## Destroy

Destroy only the VM domain:

```bash
terraform destroy -target=libvirt_domain.vm1
```

Destroy everything managed by this configuration:

```bash
terraform destroy
```

## Current Status

Verified working:

```text
Terraform                    OK
libvirt provider             OK
QEMU/KVM                     OK
storage pool                 OK
Debian QCOW2 boot            OK
cloud-init                   OK
DHCP                         OK
SSH                          OK
Terraform IP discovery       OK
Ansible hosts.ini generation OK
```

Next steps will focus on Ansible and Testinfra.
