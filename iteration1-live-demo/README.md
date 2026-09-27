# Iteration 1 Live Demo

## Setup

From ~/terraform-lab:

1. Build provider:
   mkdir -p .provider-dev
   cd terraform-provider-libvirt
   go build -o ../.provider-dev/terraform-provider-libvirt
   cd ..

2. Create local provider config:
   printf 'provider_installation {\n  dev_overrides {\n    "dmacvicar/libvirt" = "/home/ashfaaq/terraform-lab/.provider-dev"\n  }\n  direct {}\n}\n' > dev.tfrc

3. Set local provider:
   export TF_CLI_CONFIG_FILE="$HOME/terraform-lab/dev.tfrc"

## Demo

cd ~/terraform-lab/iteration1-live-demo
terraform init
terraform validate
terraform plan
terraform apply -auto-approve

virsh -c qemu:///system nwfilter-list
virsh -c qemu:///system nwfilter-dumpxml terraform-iteration1-icmp

terraform plan

terraform destroy -auto-approve

virsh -c qemu:///system nwfilter-list | grep terraform-iteration1-icmp || echo "filter successfully removed"
