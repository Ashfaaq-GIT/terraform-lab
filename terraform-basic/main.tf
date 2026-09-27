terraform {
  required_version = ">= 1.0"
}

resource "terraform_data" "test" {
  input = "Terraform is working"
}

output "message" {
  value = terraform_data.test.output
}
