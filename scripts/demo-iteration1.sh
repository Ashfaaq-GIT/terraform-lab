#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

PROVIDER_DIR="$ROOT/terraform-provider-libvirt"
PROVIDER_DEV="$ROOT/.provider-dev"
DEMO_DIR="$ROOT/iteration1-live-demo"
TFRC="$ROOT/dev.tfrc"

FILTER_NAME="terraform-iteration1-icmp"
URI="qemu:///system"
ACTION="${1:-status}"

tf() {
    TF_CLI_CONFIG_FILE="$TFRC" terraform -chdir="$DEMO_DIR" "$@"
}

setup_provider() {
    echo
    echo "===== BUILD CUSTOM PROVIDER ====="

    mkdir -p "$PROVIDER_DEV"

    (
        cd "$PROVIDER_DIR"
        go build -o "$PROVIDER_DEV/terraform-provider-libvirt"
    )

    ls -lh "$PROVIDER_DEV/terraform-provider-libvirt"

    echo
    echo "===== CREATE DEV OVERRIDE ====="

    printf 'provider_installation {\n  dev_overrides {\n    "dmacvicar/libvirt" = "%s"\n  }\n  direct {}\n}\n' \
        "$PROVIDER_DEV" > "$TFRC"

    cat "$TFRC"
}

apply_filter() {
    echo
    echo "===== ITERATION 1 TERRAFORM CONFIG ====="
    cat "$DEMO_DIR/main.tf"

    echo
    echo "===== TERRAFORM INIT ====="
    tf init

    echo
    echo "===== TERRAFORM VALIDATE ====="
    tf validate

    echo
    echo "===== TERRAFORM PLAN ====="
    tf plan

    echo
    echo "===== TERRAFORM APPLY ====="
    tf apply -auto-approve
}

verify_filter() {
    echo
    echo "===== LIBVIRT NWFILTER LIST ====="
    virsh -c "$URI" nwfilter-list

    echo
    echo "===== LIVE NWFILTER XML ====="
    virsh -c "$URI" nwfilter-dumpxml "$FILTER_NAME"
}

refresh_filter() {
    echo
    echo "===== TERRAFORM READ / REFRESH ====="
    tf plan
}

destroy_filter() {
    echo
    echo "===== TERRAFORM DESTROY ====="
    tf destroy -auto-approve

    echo
    echo "===== VERIFY CLEANUP ====="

    virsh -c "$URI" nwfilter-list \
        | grep "$FILTER_NAME" \
        || echo "filter successfully removed"
}

case "$ACTION" in
    setup)
        setup_provider
        ;;

    apply)
        apply_filter
        ;;

    verify)
        verify_filter
        ;;

    refresh)
        refresh_filter
        ;;

    status)
        virsh -c "$URI" nwfilter-list
        ;;

    destroy)
        destroy_filter
        ;;

    all)
        setup_provider
        apply_filter
        verify_filter
        refresh_filter
        destroy_filter
        ;;

    *)
        echo "Usage:"
        echo "  $0 setup"
        echo "  $0 apply"
        echo "  $0 verify"
        echo "  $0 refresh"
        echo "  $0 status"
        echo "  $0 destroy"
        echo "  $0 all"
        exit 1
        ;;
esac
