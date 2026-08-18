variable "name" {
  type = string
}

resource "netmaker_network" "test" {
  name          = var.name
  address_range = "10.51.0.0/16"
}

resource "netmaker_enrollment_key" "test" {
  networks = [netmaker_network.test.name]
  tags     = ["tf-test"]
  type     = "unlimited"
}
