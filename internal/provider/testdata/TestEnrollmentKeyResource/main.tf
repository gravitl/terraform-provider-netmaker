variable "name" {
  type = string
}

resource "netmaker_network" "test" {
  name          = var.name
  address_range = "10.51.0.0/16"
}

resource "netmaker_tag" "test" {
  network = netmaker_network.test.name
  name    = "tf-test"
}

resource "netmaker_enrollment_key" "test" {
  name     = "${var.name}-key"
  networks = [netmaker_network.test.name]
  tags     = [netmaker_tag.test.id]
  type     = "unlimited"
}
