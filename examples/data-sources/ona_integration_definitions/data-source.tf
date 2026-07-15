data "ona_integration_definitions" "available" {}

locals {
  integration_definitions_by_id = {
    for definition in data.ona_integration_definitions.available.definitions :
    definition.id => definition
  }
}
