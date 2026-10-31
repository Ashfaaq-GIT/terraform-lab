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
  name = "team5-nwfilter-policy"

  entries = [{
    rule = {
      action    = "drop"
      direction = "out"
      priority  = 100

      icmp = {
        type = 8
        code = 0
      }
    }
  }]
}

output "filter_uuid" {
  value = libvirt_nwfilter.security_policy.id
}
