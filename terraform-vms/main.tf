terraform {
  required_version = ">= 1.0"

  required_providers {
    libvirt = {
      source  = "dmacvicar/libvirt"
      version = "= 0.9.9"
    }

    local = {
      source  = "hashicorp/local"
      version = "~> 2.5"
    }
  }
}

provider "libvirt" {
  uri = "qemu:///system?socket=/run/libvirt/virtqemud-sock"
}

resource "libvirt_pool" "vm_pool" {
  name = "terraform-vms"
  type = "dir"

  target = {
    path = "/var/lib/libvirt/images/terraform-vms"
  }

  create = {
    build     = true
    start     = true
    autostart = true
  }
}

resource "libvirt_volume" "debian_base" {
  name = "debian-13-base.qcow2"
  pool = libvirt_pool.vm_pool.name

  target = {
    format = {
      type = "qcow2"
    }
  }

  create = {
    content = {
      url = abspath("${path.module}/images/debian-13-generic-amd64.qcow2")
    }
  }
}

resource "libvirt_volume" "vm1_disk" {
  name     = "vm1.qcow2"
  pool     = libvirt_pool.vm_pool.name
  capacity = 20 * 1024 * 1024 * 1024

  target = {
    format = {
      type = "qcow2"
    }
  }

  backing_store = {
    path = libvirt_volume.debian_base.path

    format = {
      type = "qcow2"
    }
  }
}

resource "libvirt_cloudinit_disk" "vm1_init" {
  name = "vm1-cloudinit"

  user_data = templatefile("${path.module}/cloud_init.cfg", {
    ssh_public_key = trimspace(file(pathexpand("~/.ssh/id_ed25519.pub")))
  })

  network_config = file("${path.module}/network_config.cfg")

  meta_data = yamlencode({
    instance-id    = "vm1"
    local-hostname = "vm1"
  })
}

resource "libvirt_volume" "vm1_cloudinit" {
  name = "vm1-cloudinit.iso"
  pool = libvirt_pool.vm_pool.name

  target = {
    format = {
      type = "iso"
    }
  }

  create = {
    content = {
      url = libvirt_cloudinit_disk.vm1_init.path
    }
  }
}

resource "libvirt_domain" "vm1" {
  name        = "vm1"
  type        = "kvm"
  memory      = 2048
  memory_unit = "MiB"
  vcpu        = 2
  running     = true
  autostart   = true

  os = {
    type         = "hvm"
    type_arch    = "x86_64"
    type_machine = "pc"
  }

  devices = {
    disks = [
      {
        device = "disk"

        driver = {
          name = "qemu"
          type = "qcow2"
        }

        source = {
          volume = {
            pool   = libvirt_volume.vm1_disk.pool
            volume = libvirt_volume.vm1_disk.name
          }
        }

        target = {
          dev = "vda"
          bus = "virtio"
        }
      },
      {
        device    = "cdrom"
        read_only = true

        source = {
          volume = {
            pool   = libvirt_volume.vm1_cloudinit.pool
            volume = libvirt_volume.vm1_cloudinit.name
          }
        }

        target = {
          dev = "hda"
          bus = "ide"
        }
      }
    ]

    interfaces = [
      {
        model = {
          type = "virtio"
        }

        source = {
          network = {
            network = "default"
          }
        }

        wait_for_ip = {
          source  = "lease"
          timeout = 300
        }
      }
    ]

    consoles = [
      {
        type = "pty"

        target = {
          type = "serial"
          port = 0
        }
      },
      {
        type = "pty"

        target = {
          type = "virtio"
          port = 1
        }
      }
    ]
  }
}

data "libvirt_domain_interface_addresses" "vm1" {
  domain = libvirt_domain.vm1.name
  source = "lease"
}

locals {
  vm1_ipv4_addresses = flatten([
    for interface in data.libvirt_domain_interface_addresses.vm1.interfaces : [
      for address in interface.addrs :
      address.addr if address.type == "ipv4"
    ]
  ])

  vm1_ip = one(local.vm1_ipv4_addresses)
}

resource "local_file" "ansible_inventory" {
  filename = "${path.module}/ansible/inventory/hosts.ini"

  content = templatefile("${path.module}/templates/hosts.yml.tftpl", {
    vm1_ip = local.vm1_ip
  })

  file_permission      = "0644"
  directory_permission = "0755"
}

output "vm1_ip" {
  value = local.vm1_ip
}

output "ansible_inventory" {
  value = local_file.ansible_inventory.filename
}
