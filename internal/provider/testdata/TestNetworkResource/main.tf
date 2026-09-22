variable "name" {
  type = string
}

variable "address_range" {
  type = string
}

resource "netmaker_network" "test" {
  name          = var.name
  address_range = var.address_range

  default_enrollment_key = {
    auto_assign_gateway = false
    tags                = ["tf-test"]
  }
}
