terraform {
  required_providers {
    libvirt = {
      source  = "dmacvicar/libvirt"
      version = "~> 0.9"
    }
  }
}

provider "libvirt" {
  uri = "qemu:///system?socket=/run/libvirt/virtqemud-sock"
}

data "libvirt_node_devices" "all" {
}

output "node_devices" {
  value = data.libvirt_node_devices.all.devices
}
