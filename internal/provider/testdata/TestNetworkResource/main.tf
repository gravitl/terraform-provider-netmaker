variable "name" {
  type = string
}

resource "netmaker_network" "test" {
  name          = var.name
  address_range = "10.50.0.0/16"

  default_enrollment_key = {
    auto_assign_gateway = false
    tags                = ["tf-test"]
  }
}
