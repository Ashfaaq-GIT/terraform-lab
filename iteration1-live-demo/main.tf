terraform {
  required_providers {
    libvirt = {
      source  = "dmacvicar/libvirt"
      version = "~> 0.9"
    }
  }
}

provider "libvirt" {
  uri = "qemu:///system"
}

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

output "nwfilter_id" {
  value = libvirt_nwfilter.icmp_demo.id
}
