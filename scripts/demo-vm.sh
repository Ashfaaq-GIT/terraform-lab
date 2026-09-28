#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
VM_DIR="$ROOT/terraform-vms"
URI="qemu:///system"
ACTION="${1:-status}"
PLAN="/tmp/terraform-vm-demo.tfplan"

unset TF_CLI_CONFIG_FILE

show_status() {
    echo
    echo "===== TERRAFORM STATE ====="
    terraform -chdir="$VM_DIR" state list || true

    echo
    echo "===== TERRAFORM OUTPUTS ====="
    terraform -chdir="$VM_DIR" output || true

    echo
    echo "===== LIBVIRT VMS ====="
    virsh -c "$URI" list --all

    echo
    echo "===== LIBVIRT NETWORKS ====="
    virsh -c "$URI" net-list --all

    echo
    echo "===== DHCP LEASES ====="
    virsh -c "$URI" net-dhcp-leases default || true
}

ssh_vm() {
    VM_IP="$(terraform -chdir="$VM_DIR" output -raw vm1_ip)"

    echo
    echo "===== SSH INTO VM ====="
    echo "VM IP: $VM_IP"
    echo "User: vmadmin"

    ssh-keygen -R "$VM_IP" >/dev/null 2>&1 || true

    echo "Waiting for SSH..."

    until ssh \
        -i "$HOME/.ssh/id_ed25519" \
        -o BatchMode=yes \
        -o StrictHostKeyChecking=accept-new \
        -o ConnectTimeout=3 \
        "vmadmin@$VM_IP" true 2>/dev/null
    do
        sleep 2
    done

    ssh \
        -i "$HOME/.ssh/id_ed25519" \
        -o StrictHostKeyChecking=accept-new \
        "vmadmin@$VM_IP"
}

case "$ACTION" in
    apply)
        echo "===== TERRAFORM VM CONFIGURATION ====="
        sed -n '1,260p' "$VM_DIR/main.tf"

        echo
        echo "===== TERRAFORM INIT ====="
        terraform -chdir="$VM_DIR" init

        echo
        echo "===== TERRAFORM VALIDATE ====="
        terraform -chdir="$VM_DIR" validate

        echo
        echo "===== TERRAFORM PLAN ====="
        terraform -chdir="$VM_DIR" plan -out="$PLAN"

        echo
        echo "===== TERRAFORM APPLY ====="
        terraform -chdir="$VM_DIR" apply "$PLAN"

        rm -f "$PLAN"

        show_status
        ;;

    status)
        show_status
        ;;

    ssh)
        ssh_vm
        ;;

    destroy)
        echo "===== TERRAFORM DESTROY ====="
        terraform -chdir="$VM_DIR" destroy -auto-approve

        echo
        echo "===== VERIFY VM CLEANUP ====="
        virsh -c "$URI" list --all

        echo
        echo "VM infrastructure demo destroyed."
        ;;

    *)
        echo "Usage:"
        echo "  $0 apply"
        echo "  $0 status"
        echo "  $0 ssh"
        echo "  $0 destroy"
        exit 1
        ;;
esac
