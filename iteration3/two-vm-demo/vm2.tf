# Iteration 3: second Debian 13 VM

resource "libvirt_volume" "vm2_disk" {
  name     = "vm2.qcow2"
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

resource "libvirt_cloudinit_disk" "vm2_init" {
  name = "vm2-cloudinit"

  user_data = templatefile("${path.module}/cloud_init.cfg", {
    ssh_public_key = trimspace(file(pathexpand("~/.ssh/id_ed25519.pub")))
  })

  network_config = file("${path.module}/network_config.cfg")

  meta_data = yamlencode({
    instance-id    = "vm2"
    local-hostname = "vm2"
  })
}

resource "libvirt_volume" "vm2_cloudinit" {
  name = "vm2-cloudinit.iso"
  pool = libvirt_pool.vm_pool.name

  target = {
    format = {
      type = "iso"
    }
  }

  create = {
    content = {
      url = libvirt_cloudinit_disk.vm2_init.path
    }
  }
}

resource "libvirt_domain" "vm2" {
  name        = "vm2"
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
            pool   = libvirt_volume.vm2_disk.pool
            volume = libvirt_volume.vm2_disk.name
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
            pool   = libvirt_volume.vm2_cloudinit.pool
            volume = libvirt_volume.vm2_cloudinit.name
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

data "libvirt_domain_interface_addresses" "vm2" {
  domain = libvirt_domain.vm2.name
  source = "lease"
}

locals {
  vm2_ipv4_addresses = flatten([
    for interface in data.libvirt_domain_interface_addresses.vm2.interfaces : [
      for address in interface.addrs :
      address.addr if address.type == "ipv4"
    ]
  ])

  vm2_ip = one(local.vm2_ipv4_addresses)
}

output "vm2_ip" {
  value = local.vm2_ip
}
