# The provider reads ONA_TOKEN by default. Use a service-account token for
# automation after the initial service-account bootstrap.
provider "ona" {}

# Set host only when using a non-default Ona API host.
# provider "ona" {
#   host = "https://app.gitpod.io"
# }
